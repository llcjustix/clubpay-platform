"""Repair only the verified Cloud API database route and admin CORS."""
import hashlib
import importlib.util
import inspect
import json
import os
import pathlib
import re
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request


OLD_ORIGIN = "https://clubpay.justix.uz"
ADMIN_ORIGIN = "https://admin-clubpay.justix.uz"
API = "https://api-clubpay.justix.uz"


def run(args, **kwargs):
    result = subprocess.run(args, capture_output=True, text=True, timeout=120, **kwargs)
    if result.returncode:
        raise RuntimeError("Scoped command failed: " + args[0])
    return result.stdout


def api_container():
    sha = os.environ["EXPECTED_API_COMMIT"]
    if not re.fullmatch("[a-f0-9]{40}", sha):
        raise ValueError("Verified deployed API SHA required")
    image = os.environ["DOCKER_USERNAME"] + "/clubpay-api:" + sha
    ids = run(["docker", "ps", "--filter", "ancestor=" + image,
               "--filter", "label=com.docker.compose.service=api", "--format", "{{.ID}}"]).split()
    if len(ids) != 1:
        raise ValueError("Expected one running API at the verified commit")
    return json.loads(run(["docker", "inspect", ids[0]]))[0], image


def connection(item):
    spec = importlib.util.spec_from_file_location("recovery", pathlib.Path(__file__).with_name("superadmin-runtime.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.connection_environment(item, os.environ)


def verify_database(item, env):
    env = dict(env, PGHOST="postgres", PGPORT="5432", PGAPPNAME="clubpay-admin-route-verification",
               PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=10000 -c lock_timeout=2000")
    h = hashlib.sha256(("clubpay-demo-salt:" + os.environ["SUPERADMIN_BOOTSTRAP_PASSWORD"]).encode()).hexdigest()
    args = ["docker", "run", "--rm", "-i", "--network", "container:" + item["Id"], "--memory", "128m", "--cpus", "0.25"]
    for key in ("PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGSSLMODE", "PGCONNECT_TIMEOUT", "PGAPPNAME", "PGOPTIONS"):
        args += ["-e", key]
    args += ["postgres:16-alpine", "psql", "-X", "--no-password", "-v", "ON_ERROR_STOP=1", "-At"]
    query = "SELECT count(*) FROM users WHERE lower(COALESCE(email,''))='superadmin@clubpay.local' AND status='active' AND global_role='super_admin' AND password_hash='" + h + "';"
    if run(args, input=query, env=env).strip() != "1":
        raise ValueError("Single active superadmin and unchanged configured password must be verified first")


def repaired_url(url):
    parts = urllib.parse.urlsplit(url)
    if parts.scheme not in ("postgres", "postgresql") or parts.path != "/clubpay":
        raise ValueError("Unexpected database scope")
    if not parts.username or not parts.password:
        raise ValueError("Incomplete database credentials")
    userinfo = parts.netloc.rsplit("@", 1)[0]
    return urllib.parse.urlunsplit((parts.scheme, userinfo + "@postgres:5432", parts.path, parts.query, parts.fragment))


def restore_api_route(item, image):
    original = dict(x.split("=", 1) for x in item["Config"]["Env"] if "=" in x)
    project = item["Config"]["Labels"].get("com.docker.compose.project", "")
    if not re.fullmatch("[a-z0-9][a-z0-9_-]*", project) or project + "_default" not in item["NetworkSettings"]["Networks"]:
        raise ValueError("Expected existing Compose network")
    env = dict(os.environ, **original)
    env.update(DB_HOST="postgres", DB_PORT="5432", IMAGE_TAG=os.environ["EXPECTED_API_COMMIT"])
    desired_url = repaired_url(original["DATABASE_URL"])
    if original["DATABASE_URL"] == desired_url and original.get("DB_HOST") == "postgres" and original.get("DB_PORT") == "5432":
        print("PASS: API already uses the verified PostgreSQL DNS route", flush=True)
        return
    override = {"services": {"api": {"environment": {"DATABASE_URL": desired_url}}},
                "networks": {"default": {"name": project + "_default", "external": True}}}
    with tempfile.TemporaryDirectory(prefix="clubpay-route-") as folder:
        path = pathlib.Path(folder) / "override.json"
        path.write_text(json.dumps(override))
        path.chmod(0o600)
        args = ["docker", "compose", "-p", project, "-f", "docker-compose.yml", "-f", str(path)]
        rendered = json.loads(run(args + ["config", "--format", "json"], env=env))
        service = rendered["services"]["api"]
        if service["image"] != image:
            raise ValueError("Refusing to change the deployed API image")
        changed_keys = {key for key, value in service["environment"].items() if original.get(key) != str(value)}
        if not changed_keys <= {"DATABASE_URL", "DB_HOST", "DB_PORT"}:
            raise ValueError("Refusing unrelated environment changes: " + ",".join(sorted(changed_keys)))
        image_config = json.loads(run(["docker", "image", "inspect", image]))[0]["Config"]
        image_env = dict(value.split("=", 1) for value in image_config.get("Env", []) if "=" in value)
        inherited_keys = {key for key, value in image_env.items() if original.get(key) == value}
        if set(original) - set(service["environment"]) - inherited_keys:
            raise ValueError("Refusing to drop existing environment settings")
        print("Verified: API image and all application settings preserved; only database host/port change", flush=True)
        run(args + ["up", "-d", "--no-deps", "--force-recreate", "--pull", "never", "api"], env=env)


def nginx_command(*args):
    return run(["docker", "run", "--rm", "--privileged", "--pid=host", "--network=none",
                "postgres:16-alpine", "nsenter", "-t", "1", "-m", "-p", "--", "/usr/sbin/nginx", *args])


def cors_patch(source):
    if not re.search(r"server_name\s+api-clubpay\.justix\.uz\s*;", source):
        raise ValueError("Unexpected Nginx virtual host")
    if "$clubpay_admin_cors_origin" in source:
        raise ValueError("CORS patch already present; inspect before retrying")
    pattern = r'add_header\s+Access-Control-Allow-Origin\s+"https://clubpay\.justix\.uz"\s+always;'
    if len(re.findall(pattern, source)) != 2:
        raise ValueError("Expected exactly two old CORS directives")
    mapping = ('map $http_origin $clubpay_admin_cors_origin {\n'
               '    default "";\n'
               '    "https://clubpay.justix.uz" "https://clubpay.justix.uz";\n'
               '    "https://admin-clubpay.justix.uz" "https://admin-clubpay.justix.uz";\n'
               '}\n\n')
    return mapping + re.sub(pattern, 'add_header Access-Control-Allow-Origin $clubpay_admin_cors_origin always;\n        add_header Vary Origin always;', source)


def restore_cors():
    nginx_command("-t")
    # Execute this module's tested pure transformation inside a narrowly
    # mounted official Python container. Preserve an exclusive rollback copy.
    run_id = os.environ["GITHUB_RUN_ID"]
    if not run_id.isdigit():
        raise ValueError("Invalid backup identifier")
    driver = '''
path = pathlib.Path('/host-nginx/conf.d/api-clubpay.justix.conf')
source = path.read_text()
updated = cors_patch(source)
backup = path.with_name(path.name + '.clubpay-recovery-' + RUN_ID + '.bak')
with backup.open('x') as stream:
    stream.write(source)
stat = path.stat()
temp = path.with_name(path.name + '.clubpay-new')
with temp.open('x') as stream:
    stream.write(updated)
os.chmod(temp, stat.st_mode)
os.chown(temp, stat.st_uid, stat.st_gid)
os.replace(temp, path)
'''
    code = "import os,pathlib,re\n" + inspect.getsource(cors_patch) + "\nRUN_ID=" + repr(run_id) + "\n" + driver
    command = ["docker", "run", "--rm", "-i", "--network=none", "--memory", "128m", "--cpus", "0.25",
               "--mount", "type=bind,source=/etc/nginx,target=/host-nginx", "python:3.12-alpine", "python", "-"]
    run(command, input=code)
    try:
        nginx_command("-t")
        nginx_command("-s", "reload")
    except Exception:
        rollback = "import pathlib,shutil; p=pathlib.Path('/host-nginx/conf.d/api-clubpay.justix.conf'); shutil.copyfile(str(p)+'.clubpay-recovery-" + run_id + ".bak',p)"
        run(command[:-1] + ["-c", rollback])
        nginx_command("-t")
        nginx_command("-s", "reload")
        raise
    print("PASS: Nginx validated and reloaded; old/new ClubPay origins allowed; rollback copy preserved", flush=True)


def verify_live():
    for origin in (ADMIN_ORIGIN, OLD_ORIGIN, "https://untrusted.example"):
        request = urllib.request.Request(API + "/api/auth/login", method="OPTIONS", headers={
            "Origin": origin, "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "content-type"})
        with urllib.request.urlopen(request, timeout=20) as response:
            actual = response.headers.get_all("Access-Control-Allow-Origin") or []
            expected = [origin] if origin in (ADMIN_ORIGIN, OLD_ORIGIN) else []
            if response.status != 204 or actual != expected:
                raise ValueError("CORS origin verification failed")
    request = urllib.request.Request(API + "/api/auth/login", data=json.dumps({
        "login": "superadmin@clubpay.local", "password": os.environ["SUPERADMIN_BOOTSTRAP_PASSWORD"]
    }).encode(), headers={"Content-Type": "application/json", "Origin": ADMIN_ORIGIN})
    token = None
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            result = json.load(response)
            token = result.get("token")
            if response.headers.get_all("Access-Control-Allow-Origin") != [ADMIN_ORIGIN]:
                raise ValueError("Login response CORS verification failed")
        if not token or result.get("user", {}).get("global_role") != "super_admin":
            raise ValueError("Actual superadmin login verification failed")
    finally:
        if token:
            logout = urllib.request.Request(API + "/api/auth/logout", data=b"{}", headers={
                "Content-Type": "application/json", "Authorization": "Bearer " + token})
            urllib.request.urlopen(logout, timeout=20).close()
    print("PASS: actual superadmin login on new origin; password unchanged; verification session revoked", flush=True)


def main():
    if os.environ.get("REPAIR_ADMIN_DOMAIN") != "true":
        raise ValueError("Explicit repair input required")
    item, image = api_container()
    verify_database(item, connection(item))
    restore_api_route(item, image)
    for attempt in range(12):
        try:
            with urllib.request.urlopen(API + "/api/health", timeout=5) as response:
                if json.load(response).get("ok"):
                    break
        except Exception:
            pass
        time.sleep(2)
    else:
        raise RuntimeError("API health did not recover")
    restore_cors()
    verify_live()


if __name__ == "__main__":
    main()
