import hashlib,json,os,re,subprocess,sys,urllib.parse,urllib.request,urllib.error

def connection_environment(item, original):
    values=dict(x.split('=',1) for x in item['Config']['Env'] if '=' in x)
    u=urllib.parse.urlsplit(values.get('DATABASE_URL',''))
    if u.scheme not in ('postgres','postgresql') or u.path!='/clubpay' or not u.hostname:raise ValueError('Unexpected running API database')
    e=dict(original);e.update(PGHOST=u.hostname,PGPORT=str(u.port or 5432),PGDATABASE='clubpay',PGUSER=urllib.parse.unquote(u.username or ''),PGPASSWORD=urllib.parse.unquote(u.password or ''),PGSSLMODE=urllib.parse.parse_qs(u.query).get('sslmode',['prefer'])[0],PGCONNECT_TIMEOUT='5',PGAPPNAME='clubpay-authorized-superadmin-recovery')
    if not e['PGUSER'] or not e['PGPASSWORD']:raise ValueError('Incomplete API connection')
    return e

def main():
    sha=os.environ['EXPECTED_API_COMMIT'];password=os.environ['SUPERADMIN_BOOTSTRAP_PASSWORD']
    if not re.fullmatch('[a-f0-9]{40}',sha) or len(password)<32:raise ValueError('Verified API SHA and strong configured secret required')
    image=os.environ['DOCKER_USERNAME']+'/clubpay-api:'+sha
    ids=subprocess.check_output(['docker','ps','--filter','ancestor='+image,'--filter','label=com.docker.compose.service=api','--format','{{.ID}}'],text=True).split()
    if len(ids)!=1:raise ValueError('Expected exactly one API at verified deployed commit')
    cid=ids[0];item=json.loads(subprocess.check_output(['docker','inspect',cid]))[0];env=connection_environment(item,os.environ)
    h=hashlib.sha256(('clubpay-demo-salt:'+password).encode()).hexdigest()
    def sql(query,readonly=False):
        e=dict(env);a=['docker','run','--rm','-i','--network','container:'+cid,'--memory','128m','--cpus','0.25']
        for k in ('PGHOST','PGPORT','PGDATABASE','PGUSER','PGPASSWORD','PGSSLMODE','PGCONNECT_TIMEOUT','PGAPPNAME'):a+=['-e',k]
        if readonly:e['PGOPTIONS']='-c default_transaction_read_only=on -c statement_timeout=15000 -c lock_timeout=2000';a+=['-e','PGOPTIONS']
        a+=['postgres:16-alpine','psql','-X','--no-password','-v','ON_ERROR_STOP=1','-At','-F','|']
        r=subprocess.run(a,input=query,text=True,capture_output=True,env=e,timeout=40)
        if r.returncode:raise RuntimeError('Scoped SQL failed; credentials withheld')
        return r.stdout.strip().splitlines()
    rows=sql("SELECT status,global_role,password_hash='"+h+"' FROM users WHERE lower(COALESCE(email,''))='superadmin@clubpay.local';",True)
    users=[dict(status=x.split('|')[0],role=x.split('|')[1],matches_secret=x.split('|')[2]=='t') for x in rows]
    same=all(env[p]==os.environ.get(g,'') for p,g in [('PGHOST','DB_HOST'),('PGPORT','DB_PORT'),('PGDATABASE','DB_NAME'),('PGUSER','DB_USER'),('PGPASSWORD','DB_PASSWORD')])
    print(json.dumps(dict(connection_source='running_api_environment',target_count=len(rows),targets=users,github_connection_matches_runtime=same)),flush=True)
    if os.environ.get('APPLY_SUPERADMIN_RESET')!='true':return
    if len(users)!=1 or users[0]['role']!='super_admin':raise ValueError('Single superadmin guard failed')
    sql("BEGIN; SET LOCAL statement_timeout='15s'; SET LOCAL lock_timeout='2s'; DO $g$ DECLARE account uuid; BEGIN SELECT id INTO STRICT account FROM users WHERE lower(COALESCE(email,''))='superadmin@clubpay.local' AND global_role='super_admin' FOR UPDATE; UPDATE users SET password_hash='"+h+"',status='active',updated_at=now() WHERE id=account; DELETE FROM auth_sessions WHERE user_id=account; END $g$; COMMIT;")
    req=urllib.request.Request('https://api-clubpay.justix.uz/api/auth/login',data=json.dumps(dict(login='superadmin@clubpay.local',password=password)).encode(),headers={'Content-Type':'application/json'})
    result=json.load(urllib.request.urlopen(req,timeout=20));user=result.get('user',{})
    if user.get('global_role')!='super_admin' or user.get('email')!='superadmin@clubpay.local' or not result.get('token'):raise RuntimeError('Actual login scope verification failed')
    req=urllib.request.Request('https://api-clubpay.justix.uz/api/auth/logout',data=b'{}',headers={'Content-Type':'application/json','Authorization':'Bearer '+result['token']});urllib.request.urlopen(req,timeout=20).close()
    print('PASS: actual superadmin login verified; verification session revoked')
if __name__=='__main__':
    try:main()
    except urllib.error.HTTPError as e:print(json.dumps(dict(error_type='HTTPError',status=e.code)));sys.exit(1)
    except Exception as e:print(json.dumps(dict(error_type=type(e).__name__,reason=str(e))));sys.exit(1)
