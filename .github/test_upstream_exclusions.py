import importlib.util
import json
from pathlib import Path
import re
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
            ('backend/internal/server/routes/user.go', 'authenticated.Group("/key-image-batches/:key_id")'),
            ('backend/internal/server/middleware/api_key_auth.go', 'strings.HasPrefix(path, "/api/v1/key-image-batches/")'),
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

    def test_primary_batch_image_source_is_preserved(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            subprocess.run(['git', 'init', '-q', str(root)], check=True)

            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()

            dedicated = 'backend/internal/service/batch_image_native.go'
            mixed = 'backend/internal/server/routes/gateway.go'
            for path, content in ((dedicated, 'native batch implementation\n'),
                                  (mixed, 'route("/images/batches", nativeHandler)\n')):
                file = root / path
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text(content)
            git('add', '--all')
            git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid',
                'commit', '-qm', 'primary native generation')
            primary = git('rev-parse', 'HEAD')
            rule = next(rule for rule in RULES['features'] if rule['name'] == '批量生图')

            def check():
                paths = set(git('ls-files', '--cached', '--others', '--exclude-standard').splitlines())
                return exclusions.check_primary_batch_image(root, paths, primary, rule)

            (root / mixed).write_text('route("/images/batches", nativeHandler)\nnewNonImageRoute()\n')
            self.assertEqual(check(), [])
            (root / mixed).write_text('route("/images/batches", forkHandler)\n')
            self.assertIn(mixed, check()[0])
            (root / mixed).write_text('route("/images/batches", nativeHandler)\n')
            (root / dedicated).unlink()
            self.assertIn(dedicated, check()[0])
            (root / dedicated).write_text('native batch implementation\n')
            fork = 'backend/internal/service/batch_image_fork.go'
            (root / fork).write_text('fork batch code\n')
            self.assertIn(fork, check()[0])

    def test_primary_batch_markers_accept_only_equivalent_wiring(self):
        rule = next(rule for rule in RULES['features'] if rule['name'] == '批量生图')

        def markers(path, source):
            return exclusions.primary_batch_markers(path, source, re.compile(rule['content_pattern'], re.I))

        client = 'backend/ent/client.go'
        native = 'c.AuthIdentity, c.BatchImageEvent, c.BatchImageItem,\n c.BatchImageJob, c.Setting,'
        merged = 'c.AuthIdentity, c.BatchImageEvent,\n c.BatchImageItem, c.BatchImageJob, c.Lottery, c.Setting,'
        self.assertEqual(markers(client, native), markers(client, merged))
        self.assertNotEqual(markers(client, native), markers(client, merged.replace('c.BatchImageJob, ', '')))
        self.assertNotEqual(markers(client, native), markers(client, merged.replace('c.Setting,', 'c.ForkBatchImageJob,')))

        group = 'backend/ent/group.go'
        native = 'case group.FieldAllowImageGeneration, group.FieldAllowBatchImageGeneration:'
        merged = 'case group.FieldStreamOnly, group.FieldAllowImageGeneration, group.FieldAllowBatchImageGeneration:'
        self.assertEqual(markers(group, native), markers(group, merged))
        self.assertNotEqual(markers(group, native), markers(group, merged.replace('FieldAllowBatchImageGeneration', 'FieldEnableBatchImageGeneration')))

        auth = 'backend/internal/service/api_key_auth_cache_impl.go'
        native = 'AllowBatchImageGeneration: apiKey.Group.AllowBatchImageGeneration,'
        merged = 'AllowBatchImageGeneration: group.AllowBatchImageGeneration,'
        self.assertEqual(markers(auth, native), markers(auth, merged))
        self.assertNotEqual(markers(auth, native), markers(auth, 'AllowBatchImageGeneration: true,'))
        self.assertNotEqual(markers(auth, native), markers(auth, merged + '\n' + merged))

        coordinator = 'backend/internal/repository/regional_funds_runtime.go'
        guard = (
            'if cfg.BatchImage.QueueEnabled {\n'
            'return nil, errors.New("batch image queue must be drained and disabled before coordinated funds")\n'
            '}'
        )
        self.assertFalse(markers(coordinator, guard))
        self.assertTrue(markers(coordinator, guard.replace('return nil, errors.New', 'log.Warn')))
        self.assertTrue(markers(coordinator, guard + '\n' + guard))

    def test_preserves_normal_images_and_negative_tests(self):
        self.assertFalse(self.check('backend/internal/server/routes/gateway.go', 'gateway.POST("/images/generations/async", handler)'))
        self.assertFalse(self.check('backend/internal/server/routes/gateway_test.go', 'assert absent /images/batches'))
        self.assertFalse(self.check('frontend/src/utils/__tests__/navigationVisibility.spec.ts', 'assert absent /batch-image'))

    def test_excludes_fork_image_surcharge_and_retains_native_pricing(self):
        self.assertTrue(self.check('backend/internal/service/image_usage_billing.go'))
        self.assertTrue(self.check('backend/internal/handler/openai_chat_completions.go', 'c.GetHeader("X-Image-Usage")'))
        self.assertTrue(self.check('backend/internal/pkg/imageeditcompat/convert.go'))
        self.assertTrue(self.check('docs/features/image-usage-billing.md'))
        self.assertFalse(self.check('backend/internal/qqbot/image_references.go', 'return imageAttachments(attachments)'))
        self.assertTrue(self.check('frontend/src/api/channels.ts', 'image_minimum_charge?: number'))
        self.assertTrue(self.check('backend/internal/service/account_cost.go', 'const ImagePurchasePricesKey = "image_purchase_prices"'))
        self.assertTrue(self.check('frontend/src/i18n/locales/zh/profitOps.ts', 'imagePurchasePrices: "legacy"'))
        self.assertTrue(self.check('backend/internal/service/usage_billing_delivery.go',
                                   'receipt := ImageBillingReceipt{}'))
        self.assertFalse(self.check('backend/internal/service/image_billing_count.go',
                                    'if group.ImageCountBilling { return count }'))
        self.assertFalse(self.check('frontend/src/api/channels.ts', 'image_output_price: number'))

    def test_preserves_qq_vision_input_but_rejects_generation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = 'backend/internal/qqbot/image_references.go'
            file = root / path
            file.parent.mkdir(parents=True)
            rules = {'features': [RULES['fork_image_generation']]}
            file.write_text('return imageAttachments(attachments)')
            self.assertEqual(exclusions.check_path(root, path, rules), [])
            file.write_text('endpoint(base, "/images/generations")')
            self.assertTrue(exclusions.check_path(root, path, rules))

    def test_shared_pool_exclusion_covers_migrations_and_embedded_wiring(self):
        for path, content in (
            ('backend/migrations/255_retire_shared_account_pool.sql', ''),
            ('backend/migrations/retire_shared_account_pool_integration_test.go', ''),
            ('backend/ent/sharedaccountwallet.go', ''),
            ('backend/internal/handler/shared_api_key_handler.go', ''),
            ('frontend/src/api/__tests__/admin.sharedPool.spec.ts', ''),
            ('backend/migrations/999_other.sql', 'ALTER TABLE shared_account_wallets'),
            ('backend/internal/service/group.go', 'IsSharedPool bool `json:"is_shared_pool"`'),
            ('backend/internal/handler/dto/types.go', 'SharedTotalCallCount int64'),
            ('backend/internal/handler/dto/types.go', 'SharedListingStatus string'),
            ('frontend/src/types/index.ts', 'shared_total_call_count?: number'),
            ('frontend/src/types/index.ts', 'shared_listing_status?: string'),
            ('backend/internal/service/group.go', 'Scope string `json:"account_scope"`'),
        ):
            with self.subTest(path=path):
                self.assertTrue(self.check(path, content))

    def test_preserves_unrelated_pools_and_account_test_picker(self):
        for path, content in (
            ('backend/internal/service/account_test_picker.go', 'type TestPickerModel struct {}'),
            ('backend/migrations/240_add_account_proxy_pool.sql', 'CREATE TABLE account_proxy_pool'),
            ('backend/internal/service/gemini_quota.go', '// Google One (shared pool)'),
            ('backend/internal/service/content_moderation.go', 'Scope string `json:"upstream_account_scope"`'),
            ('backend/migrations/258_quality_observation_scope.sql', 'CREATE UNIQUE INDEX scheduled_test_quality_account_scope_unique'),
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
        self.assertIn('生图二开排除路径', output)
        self.assertIn('frontend/src/api/batchImage.ts', output)


if __name__ == '__main__':
    unittest.main()
