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
    query = "SELECT status,global_role,password_hash='" + password_hash + "' FROM users WHERE lower(COALESCE(email,''))='superadmin@clubpay.local';"
    result = subprocess.run(command, input=query, text=True, capture_output=True, env=env, timeout=30)
    if result.returncode:
        report(database_check="failed", error_kind=sql_error_kind(result.stderr))
    else:
        rows = [row.split("|") for row in result.stdout.strip().splitlines()]
        report(database_check="ok", target_count=len(rows), targets=[
            dict(status=row[0], role=row[1], matches_configured_secret=row[2] == "t") for row in rows
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


if __name__ == "__main__":
    main()
