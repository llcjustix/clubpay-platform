import importlib.util
import pathlib
import unittest

spec = importlib.util.spec_from_file_location("repair", pathlib.Path(__file__).with_name("repair-admin-domain.py"))
repair = importlib.util.module_from_spec(spec)
spec.loader.exec_module(repair)


class AdminDomainRepairTests(unittest.TestCase):
    def test_cors_keeps_old_origin_and_denies_unlisted_origins(self):
        source = '''server {
 server_name api-clubpay.justix.uz;
 location / {
  if ($request_method = OPTIONS) {
   add_header Access-Control-Allow-Origin "https://clubpay.justix.uz" always;
   return 204;
  }
  proxy_pass http://127.0.0.1:2045;
  add_header Access-Control-Allow-Origin "https://clubpay.justix.uz" always;
 }
}'''
        patched = repair.cors_patch(source)
        self.assertIn('default "";', patched)
        for origin in (repair.OLD_ORIGIN, repair.ADMIN_ORIGIN):
            self.assertIn('"' + origin + '" "' + origin + '";', patched)
        self.assertEqual(patched.count('add_header Access-Control-Allow-Origin $clubpay_admin_cors_origin always;'), 2)
        self.assertIn('proxy_pass http://127.0.0.1:2045;', patched)
        with self.assertRaises(ValueError):
            repair.cors_patch(patched)
        with self.assertRaises(ValueError):
            repair.cors_patch(source.replace('api-clubpay.justix.uz', 'other.example'))
        with self.assertRaises(ValueError):
            repair.cors_patch(source.replace('always;', ''))

    def test_database_route_preserves_encoded_credentials_and_options(self):
        before = 'postgres://user%40tenant:fixture%3A%40%2F@old.example:6543/clubpay?sslmode=require&connect_timeout=5'
        self.assertEqual(repair.repaired_url(before), 'postgres://user%40tenant:fixture%3A%40%2F@postgres:5432/clubpay?sslmode=require&connect_timeout=5')
        with self.assertRaises(ValueError):
            repair.repaired_url(before.replace('/clubpay', '/another_database'))


if __name__ == "__main__":
    unittest.main()
