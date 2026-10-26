"""Read-only checks for the ClubPay admin domain migration."""
import hashlib
import importlib.util
import json
import os
import pathlib
import re
import shutil
import subprocess


def report(**values):
    print(json.dumps(values), flush=True)


def sql_error_kind(message):
    value = message.lower()
    for marker, kind in [
        ("password authentication failed", "password_authentication_failed"),
        ("no pg_hba.conf entry", "pg_hba_rejected"),
        ("connection refused", "connection_refused"),
        ("timeout expired", "connection_timeout"),
        ("could not translate host name", "dns_failure"),
        ("does not exist", "missing_database_or_relation"),
        ("permission denied", "permission_denied"),
    ]:
        if marker in value:
            return kind
    return "other_sql_error"


def main():
    sha = os.environ["EXPECTED_API_COMMIT"]
    if not re.fullmatch("[a-f0-9]{40}", sha):
        raise ValueError("Verified deployed API SHA required")
    image = os.environ["DOCKER_USERNAME"] + "/clubpay-api:" + sha
    ids = subprocess.check_output([
        "docker", "ps", "--filter", "ancestor=" + image,
        "--filter", "label=com.docker.compose.service=api", "--format", "{{.ID}}"
    ], text=True).split()
    if len(ids) != 1:
        raise ValueError("Expected exactly one API at verified deployed commit")
    cid = ids[0]
    item = json.loads(subprocess.check_output(["docker", "inspect", cid]))[0]
    spec = importlib.util.spec_from_file_location(
        "recovery", pathlib.Path(__file__).with_name("superadmin-runtime.py")
    )
    recovery = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(recovery)
    env = recovery.connection_environment(item, os.environ)
    env["PGAPPNAME"] = "clubpay-admin-domain-readonly"
    env["PGOPTIONS"] = "-c default_transaction_read_only=on -c statement_timeout=10000 -c lock_timeout=2000"
    command = ["docker", "run", "--rm", "-i", "--network", "container:" + cid,
               "--memory", "128m", "--cpus", "0.25"]
    for key in ("PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGSSLMODE", "PGCONNECT_TIMEOUT", "PGAPPNAME", "PGOPTIONS"):
        command += ["-e", key]
    command += ["postgres:16-alpine", "psql", "-X", "--no-password", "-v", "ON_ERROR_STOP=1", "-At", "-F", "|"]
    password_hash = hashlib.sha256(("clubpay-demo-salt:" + os.environ["SUPERADMIN_BOOTSTRAP_PASSWORD"]).encode()).hexdigest()
    requested_hash = os.environ.get("VERIFY_PASSWORD_HASH", "")
    if requested_hash:
        if not re.fullmatch("[a-f0-9]{64}", requested_hash):
            raise ValueError("Invalid verification hash")
        print("::add-mask::" + requested_hash, flush=True)
    query = "SELECT status,global_role,password_hash='" + password_hash + "',password_hash='" + requested_hash + "' FROM users WHERE lower(COALESCE(email,''))='superadmin@clubpay.local';"
    result = subprocess.run(command, input=query, text=True, capture_output=True, env=env, timeout=30)
    if result.returncode:
        report(database_check="failed", error_kind=sql_error_kind(result.stderr))
    else:
        rows = [row.split("|") for row in result.stdout.strip().splitlines()]
        report(database_check="ok", target_count=len(rows), targets=[
            dict(status=row[0], role=row[1], matches_configured_secret=row[2] == "t") for row in rows
        ])
    # Check the known host PostgreSQL service, without printing connection
    # addresses or credentials and without selecting unrelated databases.
    db_ids = subprocess.check_output([
        "docker", "ps", "--filter", "name=^/postgres$", "--format", "{{.ID}}"
    ], text=True).split()
    report(host_postgres_count=len(db_ids))
    if len(db_ids) == 1:
        db_item = json.loads(subprocess.check_output(["docker", "inspect", db_ids[0]]))[0]
        report(postgres_running=db_item["State"]["Running"],
               postgres_health=db_item["State"].get("Health", {}).get("Status"),
               shared_network_count=len(set(item["NetworkSettings"]["Networks"]) & set(db_item["NetworkSettings"]["Networks"])))
        candidates = [("inside_postgres", "container:" + db_ids[0], "127.0.0.1", "5432"),
                      ("postgres_dns", "container:" + cid, "postgres", "5432")]
        for name, network in item["NetworkSettings"]["Networks"].items():
            peer = db_item["NetworkSettings"]["Networks"].get(name)
            if peer:
                candidates.append(("shared_network", "container:" + cid, peer["IPAddress"], "5432"))
                report(shared_network=name, postgres_dns_alias_present="postgres" in (peer.get("Aliases") or []),
                       compose_managed_network=(name == item["Config"]["Labels"].get("com.docker.compose.project", "") + "_default"),
                       runtime_uses_peer_ip=(env["PGHOST"] == peer["IPAddress"]),
                       runtime_uses_internal_port=(env["PGPORT"] == "5432"))
            for port in db_item["NetworkSettings"]["Ports"].get("5432/tcp") or []:
                if network.get("Gateway") and port["HostIp"] in ("0.0.0.0", ""):
                    candidates.append(("host_gateway", "container:" + cid, network["Gateway"], port["HostPort"]))
        for label, net, host, port in candidates:
            alternate = dict(env, PGHOST=host, PGPORT=port)
            trial = list(command)
            trial[trial.index("--network") + 1] = net
            check = subprocess.run(trial, input=query, text=True, capture_output=True, env=alternate, timeout=25)
            report(database_route=label, works=check.returncode == 0,
                   error_kind=sql_error_kind(check.stderr) if check.returncode else None,
                   matches_runtime_endpoint=(host == env["PGHOST"] and port == env["PGPORT"]))
            if not check.returncode:
                checked_rows = [line.split("|") for line in check.stdout.strip().splitlines()]
                report(database_route_account=label, target_count=len(checked_rows), targets=[
                    dict(status=row[0], role=row[1], matches_configured_secret=row[2] == "t",
                         matches_requested_password=row[3] == "t") for row in checked_rows
                ])
    report(runner_nginx_available=bool(shutil.which("nginx")), runner_sudo_available=bool(shutil.which("sudo")), runner_uid=os.geteuid())
    # The runner may be in a container. Read only the host's Nginx directory;
    # never print other virtual hosts, keys, or complete configuration files.
    scan = subprocess.run([
        "docker", "run", "--rm", "--network", "none", "--memory", "64m", "--cpus", "0.25",
        "--mount", "type=bind,source=/etc/nginx,target=/host-nginx,readonly",
        "postgres:16-alpine", "sh", "-c",
        "find /host-nginx -type f -size -256k -exec sh -c 'for f do if grep -q api-clubpay.justix.uz \"$f\"; then printf \"FILE:%s\\n\" \"$f\"; sed -n \"1,240p\" \"$f\"; fi; done' sh {} +"
    ], capture_output=True, text=True, timeout=30)
    if scan.returncode:
        report(nginx_host_scan="unavailable")
    else:
        selected = []
        for line in scan.stdout.splitlines():
            if line.startswith("FILE:") or re.search(r"server_name|listen |proxy_pass|Access-Control-|request_method|add_header|proxy_hide_header|include |^\s*(map|location|if)\b", line):
                if not re.search(r"Authorization|[Ss]ecret|[Pp]assword|Bearer|[Tt]oken", line) or "Access-Control-" in line:
                    selected.append(line)
        report(nginx_host_scan="ok", relevant_directives=selected)
    native_test = subprocess.run([
        "docker", "run", "--rm", "--privileged", "--pid=host", "--network=none",
        "postgres:16-alpine", "nsenter", "-t", "1", "-m", "-p", "--", "/usr/sbin/nginx", "-t"
    ], capture_output=True, text=True, timeout=20)
    report(host_nginx_test_ok=native_test.returncode == 0,
           diagnostic=native_test.stderr[-1000:] if native_test.returncode else "configuration valid")


if __name__ == "__main__":
    main()
