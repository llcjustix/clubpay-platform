import importlib.util,pathlib,unittest
spec=importlib.util.spec_from_file_location('recovery',pathlib.Path(__file__).with_name('superadmin-runtime.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class RuntimeAuthorityTests(unittest.TestCase):
 def test_running_api_connection_wins_over_stale_github_settings(self):
  item={'Config':{'Env':['DATABASE_URL=postgres://runtime:fixture%21@db.internal:5433/clubpay?sslmode=disable']}}
  e=m.connection_environment(item,{'DB_HOST':'stale','DB_NAME':'obsolete'})
  self.assertEqual((e['PGHOST'],e['PGPORT'],e['PGDATABASE'],e['PGUSER'],e['PGPASSWORD']),('db.internal','5433','clubpay','runtime','fixture!'))
 def test_other_database_is_rejected_without_leaking_connection(self):
  item={'Config':{'Env':['DATABASE_URL=postgres://runtime:fixture%21@db.internal/anotherdb']}}
  with self.assertRaises(ValueError) as error:m.connection_environment(item,{})
  self.assertNotIn('fixture',str(error.exception));self.assertNotIn('postgres://',str(error.exception))
 def test_incomplete_connection_is_rejected(self):
  with self.assertRaises(ValueError):m.connection_environment({'Config':{'Env':['DATABASE_URL=postgres://runtime@db.internal/clubpay']}},{})
if __name__=='__main__':unittest.main()
