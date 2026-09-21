import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('exclusions', HERE / 'check-upstream-exclusions.py')
exclusions = importlib.util.module_from_spec(spec)
spec.loader.exec_module(exclusions)
RULES = json.loads((HERE / 'upstream-exclusions.json').read_text())


class UpstreamExclusionsTest(unittest.TestCase):
    def check(self, path, content=''):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            file = root / path
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text(content)
            return exclusions.check_path(root, path, RULES)

    def test_rejects_dedicated_feature_files(self):
        for path in ('frontend/src/views/user/BatchImageGuideView.vue',
                     'backend/internal/service/batch_image_worker.go',
                     'backend/ent/batchimagejob.go',
                     'frontend/src/composables/useBatchImageAccess.ts',
                     'backend/internal/service/shared_pool.go'):
            with self.subTest(path=path):
                self.assertTrue(self.check(path))

    def test_rejects_reintroduced_routes_and_settings_in_shared_files(self):
        for path, content in (
            ('frontend/src/utils/navigationVisibility.ts', "{ path: '/batch-image' }"),
            ('backend/internal/server/routes/gateway.go', 'gateway.POST("/images/batches", handler)'),
            ('backend/internal/config/config.go', 'BatchImage BatchImageConfig'),
            ('deploy/docker-compose.yml', 'BATCH_IMAGE_ENABLED=true'),
        ):
            with self.subTest(path=path):
                self.assertTrue(self.check(path, content))

    def test_keeps_shipped_migrations_but_rejects_new_feature_migrations(self):
        self.assertFalse(self.check('backend/migrations/159_batch_image_foundation.sql', 'CREATE TABLE batch_image_jobs'))
        self.assertFalse(self.check('backend/migrations/187_add_usage_log_session_id.sql', 'ALTER TABLE batch_image_jobs'))
        self.assertTrue(self.check('backend/migrations/999_batch_image_new.sql'))
        self.assertTrue(self.check('backend/migrations/999_other.sql', 'ALTER TABLE batch_image_jobs'))

    def test_preserves_normal_images_and_negative_tests(self):
        self.assertFalse(self.check('backend/internal/server/routes/gateway.go', 'gateway.POST("/images/generations/async", handler)'))
        self.assertFalse(self.check('backend/internal/server/routes/gateway_test.go', 'assert absent /images/batches'))
        self.assertFalse(self.check('frontend/src/utils/__tests__/navigationVisibility.spec.ts', 'assert absent /batch-image'))

    def test_shared_pool_exclusion_covers_migrations_and_embedded_wiring(self):
        for path, content in (
            ('backend/migrations/255_retire_shared_account_pool.sql', ''),
            ('backend/migrations/retire_shared_account_pool_integration_test.go', ''),
            ('backend/ent/sharedaccountwallet.go', ''),
            ('backend/internal/handler/shared_api_key_handler.go', ''),
            ('frontend/src/api/__tests__/admin.sharedPool.spec.ts', ''),
            ('backend/migrations/999_other.sql', 'ALTER TABLE shared_account_wallets'),
            ('backend/internal/service/group.go', 'IsSharedPool bool `json:"is_shared_pool"`'),
        ):
            with self.subTest(path=path):
                self.assertTrue(self.check(path, content))

    def test_preserves_unrelated_pools_and_account_test_picker(self):
        for path, content in (
            ('backend/internal/service/account_test_picker.go', 'type TestPickerModel struct {}'),
            ('backend/migrations/240_add_account_proxy_pool.sql', 'CREATE TABLE account_proxy_pool'),
            ('backend/internal/service/gemini_quota.go', '// Google One (shared pool)'),
            ('backend/internal/service/token_refresh_pool_health_test.go', 'sharedPoolGate := pool'),
        ):
            with self.subTest(path=path):
                self.assertFalse(self.check(path, content))

    def test_merge_report_includes_retired_feature_paths(self):
        decision = {
            'decision': 'resolved', 'summary': '功能排除完成',
            'upstream_changes': [], 'impacts': [], 'merged_behavior': [],
            'conflicts': [], 'risks': [], 'excluded_shared_account_pool_paths': [],
            'excluded_batch_image_paths': ['frontend/src/api/batchImage.ts'],
        }
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'decision.json'
            path.write_text(json.dumps(decision))
            output = subprocess.check_output([
                sys.executable, str(HERE / 'render-upstream-sync-review.py'), str(path),
            ], text=True)
        self.assertIn('批量生图排除路径', output)
        self.assertIn('frontend/src/api/batchImage.ts', output)


if __name__ == '__main__':
    unittest.main()
