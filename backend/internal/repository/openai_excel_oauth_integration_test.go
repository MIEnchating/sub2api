//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func excelAuthorizationFixture(t *testing.T) (context.Context, *openAIOAuthReauthRepository, *service.Account) {
	t.Helper()
	ctx := context.Background()
	account := mustCreateAccount(t, testEntClient(t), &service.Account{
		Name: fmt.Sprintf("excel-auth-%d", time.Now().UnixNano()), Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "synthetic-codex", "chatgpt_account_id": "workspace-a", "email": "fixture@example.test", "plan_type": "plus"},
		Extra:       map[string]any{"openai_excel_bps": true, service.ExcelBPSAuthorizationPendingKey: true},
	})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id=$1 OR payload->'account_ids' @> jsonb_build_array($1::bigint)", account.ID)
		require.NoError(t, err)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	repo := &openAIOAuthReauthRepository{db: integrationDB}
	err := repo.UpsertConfig(ctx, &service.OpenAIOAuthReauthStoredConfig{AccountID: account.ID, LoginEmail: "fixture@example.test", CredentialMode: "password_totp", Engine: "local_worker", ProxySource: "account", PasswordCiphertext: "synthetic-encrypted-login"})
	require.NoError(t, err)
	return ctx, repo, account
}

func TestExcelAuthorizationGrantTransactionPreservesPrimaryAccount(t *testing.T) {
	ctx, repo, account := excelAuthorizationFixture(t)
	accounts := NewAccountRepository(integrationEntClient, integrationDB, nil)
	_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET status='error',schedulable=false WHERE id=$1`, account.ID)
	require.NoError(t, err)
	before, err := accounts.GetByID(ctx, account.ID)
	require.NoError(t, err)
	task, err := repo.CreateExcelTask(ctx, account.ID, "snapshot")
	require.NoError(t, err)
	_, err = repo.CreateExcelTask(ctx, account.ID, "duplicate")
	require.Error(t, err)
	claimed, err := repo.ClaimNextTaskForEngines(ctx, "excel-fixture-worker", time.Minute, "password_totp", []string{"local_worker", "session_studio"})
	require.NoError(t, err)
	require.Equal(t, task.ID, claimed.ID)
	claimed, started, err := repo.BeginDirectCallback(ctx, task.ID, "excel-fixture-worker")
	require.NoError(t, err)
	require.True(t, started)
	changed, err := repo.ApplyExcelCredentials(ctx, claimed, map[string]any{"access_token": "stale"}, "encrypted-excel")
	require.NoError(t, err)
	require.False(t, changed)
	ciphertext, err := repo.GetExcelCredentials(ctx, account.ID)
	require.NoError(t, err)
	require.Empty(t, ciphertext)
	changed, err = repo.ApplyExcelCredentials(ctx, claimed, before.Credentials, "encrypted-excel", service.ExcelBPSGrantState(time.Now().Add(time.Hour).Unix()))
	require.NoError(t, err)
	require.True(t, changed)
	after, err := accounts.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, before.Credentials, after.Credentials)
	require.Equal(t, before.Schedulable, after.Schedulable)
	require.Equal(t, before.Status, after.Status)
	require.True(t, after.IsExcelBPSEnabled())
	require.NotEqual(t, true, after.Extra[service.ExcelBPSAuthorizationPendingKey])
	completed, err := repo.GetTask(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", completed.Status)
	changed, err = repo.ReplaceExcelCredentials(ctx, account.ID, "encrypted-excel", "encrypted-newer")
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, repo.DeleteExcelCredentials(ctx, account.ID, "encrypted-excel"))
	ciphertext, err = repo.GetExcelCredentials(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "encrypted-newer", ciphertext)
}

func TestExcelAuthorizationBulkPendingIsPerAccount(t *testing.T) {
	ctx, repo, missing := excelAuthorizationFixture(t)
	_, _, ready := excelAuthorizationFixture(t)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO openai_excel_oauth_credentials(account_id,credentials_ciphertext) VALUES($1,'synthetic-encrypted-grant')`, ready.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra=extra||'{"openai_excel_bps_credential_state":{"status":"not_expired"}}'::jsonb WHERE id=$1`, ready.ID)
	require.NoError(t, err)
	accounts := NewAccountRepository(integrationEntClient, integrationDB, nil)
	_, err = accounts.BulkUpdate(ctx, []int64{missing.ID, ready.ID}, service.AccountBulkUpdate{Extra: map[string]any{"openai_excel_bps": true, "unrelated": "keep", service.ExcelBPSAuthorizationPendingKey: false}})
	require.NoError(t, err)
	missingAfter, err := accounts.GetByID(ctx, missing.ID)
	require.NoError(t, err)
	readyAfter, err := accounts.GetByID(ctx, ready.ID)
	require.NoError(t, err)
	require.Equal(t, true, missingAfter.Extra[service.ExcelBPSAuthorizationPendingKey])
	require.False(t, missingAfter.IsExcelBPSEnabled())
	require.NotEqual(t, true, readyAfter.Extra[service.ExcelBPSAuthorizationPendingKey])
	require.True(t, readyAfter.IsExcelBPSEnabled())
	// Simulate an edit prepared before the grant completed. Full Update preserves
	// server diagnosis and must not reintroduce its old preparation marker.
	readyAfter.Extra[service.ExcelBPSCredentialStateExtraKey] = map[string]any{"status": "forged"}
	readyAfter.Extra[service.ExcelBPSAuthorizationPendingKey] = true
	require.NoError(t, accounts.Update(ctx, readyAfter))
	readyAfter, err = accounts.GetByID(ctx, ready.ID)
	require.NoError(t, err)
	require.True(t, readyAfter.IsExcelBPSEnabled())
	require.Equal(t, "not_expired", readyAfter.Extra[service.ExcelBPSCredentialStateExtraKey].(map[string]any)["status"])

	require.Equal(t, "keep", missingAfter.Extra["unrelated"])
	require.Equal(t, "keep", readyAfter.Extra["unrelated"])
	ids, err := repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.Contains(t, ids, missing.ID)
	require.NotContains(t, ids, ready.ID)
	require.NoError(t, accounts.UpdateExtra(ctx, ready.ID, map[string]any{"openai_excel_bps": true, service.ExcelBPSAuthorizationPendingKey: true}))
	readyAfter, err = accounts.GetByID(ctx, ready.ID)
	require.NoError(t, err)
	require.True(t, readyAfter.IsExcelBPSEnabled())
	task, err := repo.CreateExcelTask(ctx, missing.ID, "snapshot")
	require.NoError(t, err)
	ids, err = repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.NotContains(t, ids, missing.ID)
	_, err = integrationDB.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET status='failed',stage='failed' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	ids, err = repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.NotContains(t, ids, missing.ID)
	_, err = integrationDB.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET updated_at=NOW()-INTERVAL '31 minutes' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	ids, err = repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.Contains(t, ids, missing.ID)
}

func TestExcelAuthorizationCompletionHonorsDisabledSwitch(t *testing.T) {
	ctx, repo, account := excelAuthorizationFixture(t)
	task, err := repo.CreateExcelTask(ctx, account.ID, "snapshot")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET status='callback_processing',worker_id='fixture' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	task.WorkerID = "fixture"
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra=extra||'{"openai_excel_bps":false}'::jsonb WHERE id=$1`, account.ID)
	require.NoError(t, err)
	changed, err := repo.ApplyExcelCredentials(ctx, task, account.Credentials, "synthetic-encrypted-grant")
	require.NoError(t, err)
	require.True(t, changed)
	accounts := NewAccountRepository(integrationEntClient, integrationDB, nil)
	updated, err := accounts.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.False(t, updated.IsExcelBPSEnabled())
	require.Equal(t, false, updated.Extra["openai_excel_bps"])
	require.NoError(t, repo.PrepareMissingExcelRoutes(ctx))
	updated, err = accounts.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.False(t, updated.IsExcelBPSEnabled())
}

func TestExcelAuthorizationMigrationIsIdempotent(t *testing.T) {
	_ = testEntClient(t)
	raw, err := os.ReadFile("../../migrations/338_excel_oauth_authorization.sql")
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	for i := 0; i < 2; i++ {
		_, err = tx.ExecContext(context.Background(), string(raw))
		require.NoError(t, err)
	}
}

func TestExcelOAuthIntegrationDualGrantAtomicityAndEngineRouting(t *testing.T) {
	ctx, repo, cfg := totpRotationFixture(t)
	accountRepo := NewAccountRepository(integrationEntClient, integrationDB, nil)
	before, err := accountRepo.GetByID(ctx, cfg.AccountID)
	require.NoError(t, err)
	encryptor, err := NewAESEncryptor(&config.Config{Totp: config.TotpConfig{EncryptionKey: strings.Repeat("42", 32)}})
	require.NoError(t, err)
	ciphertext, err := encryptor.Encrypt(`{"access_token":"fixture-excel-token"}`)
	require.NoError(t, err)
	require.NotContains(t, ciphertext, "fixture-excel-token")

	// Local Excel tasks override a Session Studio global preference. Codex keeps it.
	task, err := repo.CreateExcelTask(ctx, cfg.AccountID, "snapshot")
	require.NoError(t, err)
	require.Equal(t, "excel", task.OAuthProfile)
	_, err = repo.CreateTask(ctx, cfg.AccountID, "snapshot")
	require.Error(t, err, "same-account login remains exclusive")
	skipped, err := repo.ClaimNextTaskForRuntime(ctx, "remote", time.Minute, "", []string{"session_studio"}, "session_studio")
	require.NoError(t, err)
	require.Nil(t, skipped)
	claimed, err := repo.ClaimNextTaskForRuntime(ctx, "local", time.Minute, "password_totp", []string{"local_worker"}, "session_studio")
	require.NoError(t, err)
	require.Equal(t, task.ID, claimed.ID)
	claimed, started, err := repo.BeginDirectCallback(ctx, task.ID, "local")
	require.NoError(t, err)
	require.True(t, started)
	require.Equal(t, "excel", claimed.OAuthProfile)
	changed, err := repo.ApplyExcelCredentials(ctx, claimed, map[string]any{"email": "stale@example.test"}, ciphertext)
	require.NoError(t, err)
	require.False(t, changed)
	empty, err := repo.GetExcelCredentials(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Empty(t, empty)
	// A failed task-finalization write rolls back the separately encrypted grant.
	_, err = integrationDB.ExecContext(ctx, `CREATE FUNCTION excel_task_fail() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'synthetic finalization failure'; END$$;
 CREATE TRIGGER excel_task_fail BEFORE UPDATE ON openai_oauth_reauth_tasks FOR EACH ROW WHEN(NEW.status='succeeded') EXECUTE FUNCTION excel_task_fail()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS excel_task_fail ON openai_oauth_reauth_tasks; DROP FUNCTION IF EXISTS excel_task_fail()`)
	})
	_, err = repo.ApplyExcelCredentials(ctx, claimed, before.Credentials, ciphertext)
	require.Error(t, err)
	empty, err = repo.GetExcelCredentials(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Empty(t, empty)
	_, err = integrationDB.ExecContext(ctx, `DROP TRIGGER excel_task_fail ON openai_oauth_reauth_tasks; DROP FUNCTION excel_task_fail()`)
	require.NoError(t, err)
	changed, err = repo.ApplyExcelCredentials(ctx, claimed, before.Credentials, ciphertext)
	require.NoError(t, err)
	require.True(t, changed)
	after, err := accountRepo.GetByID(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Equal(t, before.Credentials, after.Credentials)
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.Schedulable, after.Schedulable)
	completed, err := repo.GetTask(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, service.OpenAIOAuthReauthStatusSucceeded, completed.Status)
	require.Equal(t, "excel", completed.OAuthProfile)
	saved, err := repo.GetExcelCredentials(ctx, cfg.AccountID)
	require.NoError(t, err)
	plain, err := encryptor.Decrypt(saved)
	require.NoError(t, err)
	require.Contains(t, plain, "fixture-excel-token")

	changed, err = repo.ReplaceExcelCredentials(ctx, cfg.AccountID, "wrong", "encrypted:second")
	require.NoError(t, err)
	require.False(t, changed)
	changed, err = repo.ReplaceExcelCredentials(ctx, cfg.AccountID, ciphertext, "encrypted:second")
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, repo.DeleteExcelCredentials(ctx, cfg.AccountID, ciphertext))
	current, err := repo.GetExcelCredentials(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Equal(t, "encrypted:second", current)
	_, err = repo.CreateTask(ctx, cfg.AccountID, "snapshot")
	require.NoError(t, err)
	codex, err := repo.ClaimNextTaskForRuntime(ctx, "remote", time.Minute, "", []string{"session_studio"}, "session_studio")
	require.NoError(t, err)
	require.NotNil(t, codex)
	require.Equal(t, "codex", codex.OAuthProfile)
}

func TestExcelOAuthIntegrationQueueCoversExistingSwitchAndCooldown(t *testing.T) {
	ctx, repo, cfg := totpRotationFixture(t)
	_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_build_object('openai_excel_bps',true) WHERE id=$1`, cfg.AccountID)
	require.NoError(t, err)
	ids, err := repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.Contains(t, ids, cfg.AccountID)
	task, err := repo.CreateExcelTask(ctx, cfg.AccountID, "snapshot")
	require.NoError(t, err)
	ids, err = repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.NotContains(t, ids, cfg.AccountID)
	_, err = repo.ClaimNextTask(ctx, "local", time.Minute)
	require.NoError(t, err)
	require.NoError(t, repo.MarkFailed(ctx, task.ID, "local", "bounded login failure"))
	ids, err = repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.NotContains(t, ids, cfg.AccountID)
	_, err = integrationDB.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET updated_at=NOW()-INTERVAL '31 minutes' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	ids, err = repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.Contains(t, ids, cfg.AccountID)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=credentials||jsonb_build_object('plan_type','free') WHERE id=$1`, cfg.AccountID)
	require.NoError(t, err)
	ids, err = repo.ListMissingExcelAuthorizations(ctx, 50)
	require.NoError(t, err)
	require.NotContains(t, ids, cfg.AccountID)
}

func TestLatestExcelAuthorizationIgnoresNewerCodexTask(t *testing.T) {
	ctx, repo, cfg := totpRotationFixture(t)
	task, err := repo.CreateExcelTask(ctx, cfg.AccountID, "test-snapshot")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET status='failed',stage='failed',error_message='reason=security_check' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	_, err = repo.CreateTask(ctx, cfg.AccountID, "new-codex-snapshot")
	require.NoError(t, err)
	latest, err := repo.GetLatestExcelTask(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Equal(t, task.ID, latest.ID)
	require.Equal(t, "failed", latest.Status)
}

func TestExcelAuthorizationAutomaticallyActivatesRequestedRoute(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "activate", true: "cancel during login"}[cancel], func(t *testing.T) {
			ctx, repo, cfg := totpRotationFixture(t)
			accountRepo := NewAccountRepository(integrationEntClient, integrationDB, nil)
			before, err := accountRepo.GetByID(ctx, cfg.AccountID)
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra=extra||'{"openai_excel_bps":true,"openai_excel_bps_authorization_pending":true}'::jsonb WHERE id=$1`, cfg.AccountID)
			require.NoError(t, err)
			task, err := repo.CreateExcelTask(ctx, cfg.AccountID, "snapshot")
			require.NoError(t, err)
			_, err = integrationDB.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET status='callback_processing',worker_id='test-worker' WHERE id=$1`, task.ID)
			require.NoError(t, err)
			task.WorkerID = "test-worker"
			if cancel {
				_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET extra=extra||'{"openai_excel_bps":false}'::jsonb WHERE id=$1`, cfg.AccountID)
				require.NoError(t, err)
			}
			changed, err := repo.ApplyExcelCredentials(ctx, task, before.Credentials, "test-encrypted-grant")
			require.NoError(t, err)
			require.True(t, changed)
			after, err := accountRepo.GetByID(ctx, cfg.AccountID)
			require.NoError(t, err)
			require.Equal(t, !cancel, after.IsExcelBPSEnabled())
			require.Equal(t, before.Credentials, after.Credentials)
			if !cancel {
				require.NotEqual(t, true, after.Extra[service.ExcelBPSAuthorizationPendingKey])
			}
		})
	}
}

func TestExistingMissingExcelRouteAutomaticallyPrepares(t *testing.T) {
	ctx, repo, cfg := totpRotationFixture(t)
	_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET extra=extra||'{"openai_excel_bps":true}'::jsonb WHERE id=$1`, cfg.AccountID)
	require.NoError(t, err)
	require.NoError(t, repo.PrepareMissingExcelRoutes(ctx))
	accountRepo := NewAccountRepository(integrationEntClient, integrationDB, nil)
	account, err := accountRepo.GetByID(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Equal(t, true, account.Extra["openai_excel_bps"])
	require.Equal(t, true, account.Extra[service.ExcelBPSAuthorizationPendingKey])
	require.False(t, account.IsExcelBPSEnabled())
	require.NoError(t, repo.PrepareMissingExcelRoutes(ctx))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO openai_excel_oauth_credentials(account_id,credentials_ciphertext) VALUES($1,'saved-test-grant')`, cfg.AccountID)
	require.NoError(t, err)
	require.NoError(t, repo.PrepareMissingExcelRoutes(ctx))
	account, err = accountRepo.GetByID(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.True(t, account.IsExcelBPSEnabled())
}

func TestBulkExcelPreparationUsesPerAccountState(t *testing.T) {
	ctx, _, first := totpRotationFixture(t)
	_, _, second := totpRotationFixture(t)
	repo := NewAccountRepository(integrationEntClient, integrationDB, nil)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO openai_excel_oauth_credentials(account_id,credentials_ciphertext) VALUES($1,'saved-test-grant')`, first.AccountID)
	require.NoError(t, err)
	_, err = repo.BulkUpdate(ctx, []int64{first.AccountID, second.AccountID}, service.AccountBulkUpdate{
		Extra:                        map[string]any{"openai_excel_bps": true},
		ExcelBPSAuthorizationPending: map[int64]bool{first.AccountID: false, second.AccountID: true},
	})
	require.NoError(t, err)
	a, err := repo.GetByID(ctx, first.AccountID)
	require.NoError(t, err)
	require.True(t, a.IsExcelBPSEnabled())
	b, err := repo.GetByID(ctx, second.AccountID)
	require.NoError(t, err)
	require.False(t, b.IsExcelBPSEnabled())
	require.Equal(t, true, b.Extra["openai_excel_bps"])
	_, err = repo.BulkUpdate(ctx, []int64{second.AccountID}, service.AccountBulkUpdate{Extra: map[string]any{"openai_excel_bps": false}, ExcelBPSAuthorizationPending: map[int64]bool{second.AccountID: false}})
	require.NoError(t, err)
	b, err = repo.GetByID(ctx, second.AccountID)
	require.NoError(t, err)
	require.NotEqual(t, true, b.Extra[service.ExcelBPSAuthorizationPendingKey])
}
