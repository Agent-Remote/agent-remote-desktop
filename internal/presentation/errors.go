// Package presentation translates service results at the user-facing boundary.
package presentation

import (
	"context"
	"errors"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"os"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

// Error resolves the language when rendered, not when the error was created.
// Unknown external errors retain their diagnostic detail below a localized summary.
func Error(err error) string {
	if err == nil {
		return ""
	}
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		switch operation.Code {
		case "sync_mutagen_environment_outcome_unknown":
			return locales.MutagenEnvironmentUnknown()
		case "sync_mutagen_environment_changed", "sync_mutagen_inventory_changed", "sync_mutagen_inventory_invalid", "sync_mutagen_inventory_limit", "sync_mutagen_daemon_busy", "sync_mutagen_daemon_lock_missing", "sync_mutagen_registration_external", "sync_mutagen_registration_unknown", "sync_mutagen_environment_unverified", "sync_mutagen_environment_write_failed", "sync_mutagen_environment_blocked":
			return locales.MutagenEnvironmentResultCode(operation.Code)
		case "sync_outcome_unknown":
			return locales.SyncUnknown()
		case "sync_recovery_not_required", "sync_recovery_blocked", "sync_recovery_metadata_incomplete", "sync_action_invalid", "sync_conflicts_remaining", "sync_conflicts_unknown", "sync_git_check_failed", "sync_git_check_limit", "sync_git_locked", "sync_mutagen_ambiguous", "sync_mutagen_environment_invalid", "sync_mutagen_environment_mismatch", "sync_mutagen_missing", "sync_mutagen_not_running", "sync_mutagen_output_limit", "sync_mutagen_proxy_missing", "sync_mutagen_response_invalid", "sync_mutagen_target_changed", "sync_mutagen_timeout", "sync_mutagen_unavailable", "sync_mutagen_unverified", "sync_target_invalid", "sync_target_changed", "sync_not_found", "sync_local_failed", "sync_prepare_failed", "sync_prepare_timeout", "sync_rejected", "sync_response_invalid", "mutagen_unavailable", "canceled":
			return locales.SyncResultCode(operation.Code)
		case "account_outcome_unknown":
			return locales.AccountOutcomeUnknown()
		case "account_rejected":
			return locales.AccountRejected()
		case "account_not_found":
			return locales.AccountNotFound()
		case "account_response_invalid":
			return locales.AccountResponseInvalid()
		case "default_outcome_unknown":
			return locales.AccountDefaultUnknown()
		case "default_changed":
			return locales.AccountDefaultChanged()
		case "account_mismatch":
			return locales.AccountDefaultMismatch()
		case "state_unavailable":
			return locales.AccountDefaultStateUnavailable()
		case "unsupported_method":
			return locales.AccountDefaultUnsupported()
		case "wireguard_outcome_unknown":
			return locales.WireGuardOutcomeUnknown()
		case "wireguard_execution_failed":
			return locales.WireGuardExecutionFailed()
		case "wireguard_precondition_failed":
			return locales.WireGuardPreconditionFailed()
		case "wireguard_plan_stale":
			return locales.WireGuardTunnelPlanStale()
		case "execution_bridge_required":
			return locales.WireGuardTunnelPrerequisite("execution_bridge_required")
		case "session_create_request_conflict":
			return locales.SessionCreateConflict()
		case "session_create_sync_target_changed":
			return locales.SessionCreateTargetChanged()
		case "session_create_submission_unknown", "session_create_outcome_unknown":
			return locales.SessionCreateUnknown()
		case "session_create_deleted":
			return locales.SessionCreateDeleted()
		case "session_create_node_mismatch":
			return locales.SessionCreateNodeMismatch()
		case "session_create_prepare_unproven", "session_create_sync_not_ready":
			return locales.SessionCreatePrepareUnproven()
		case "session_stop_outcome_unknown":
			return locales.RemoteStopUnknown()
		case "SESSION_STOP_SUBMISSION_UNKNOWN":
			return locales.RemoteStopUnknown()
		case "session_binding_changed":
			return locales.RemoteContextChanged()
		case "SESSION_STOP_UNRESOLVED", "session_stop_pending", "session_stop_status_unavailable", "session_stop_response_invalid":
			return locales.RemoteStopFailed()
		case "session_delete_request_conflict", "SESSION_DELETE_REQUEST_CONFLICT", "SESSION_DELETE_COLLECTION_MISMATCH", "SESSION_DELETE_CONTEXT_MISMATCH":
			return locales.RemoteDeleteConflict()
		case "session_delete_outcome_unknown", "session_delete_status_unavailable", "SESSION_DELETE_SUBMISSION_UNKNOWN":
			return locales.RemoteDeleteUnknown()
		case "session_delete_pending":
			return locales.RemoteDeletePending()
		case "SESSION_DELETE_UNRESOLVED", "session_delete_failed":
			return locales.RemoteDeleteFailed()
		case "output_invalid", "output_unavailable", "output_changed", "operation_busy", "wireguard_key_invalid", "wireguard_key_store_incomplete", "wireguard_enrollment_rejected", "wireguard_enrollment_unknown", "wireguard_config_unavailable", "wireguard_config_invalid", "wireguard_context_changed", "wireguard_publication_incomplete":
			return locales.WireGuardResultCode(operation.Code)
		case "setup_outcome_unknown":
			return locales.LocalSetupUnknown()
		case "logout_outcome_unknown":
			return locales.LogoutOutcomeUnknown()
		case "configuration_changed":
			return locales.RemoteContextChanged()
		case "login_expired":
			return locales.LoginExpired()
		case "login_rejected":
			return locales.LoginRejected()
		case "registration_rejected":
			return locales.LoginRegistrationRejected()
		case "login_outcome_unknown", "registration_outcome_unknown":
			return locales.LoginOutcomeUnknown()
		case "login_persistence_incomplete":
			return locales.LoginPersistenceIncomplete()
		case "ssh_public_key_required":
			return locales.LoginPublicKeyRequired()
		case "invalid_params":
			return locales.LoginInvalidParams()
		case "confirmation_required":
			return locales.LoginConfirmationRequired()
		case "authentication_required", "credential_unavailable":
			return locales.RemoteAuthenticationRequired()
		case "permission_denied":
			return locales.RemotePermissionDenied()
		case "configuration_unavailable":
			return locales.RemoteConfigurationUnavailable()
		case "server_unavailable", "rate_limited", "timeout":
			return locales.RemoteServerUnavailable()
		default:
			return locales.ErrorCLIProtocol()
		}
	}
	for _, entry := range []struct {
		code error
		text func() string
	}{
		{fault.AccountInputInvalid, locales.AccountInputInvalid},
		{fault.AccountOutcomeUnknown, locales.AccountOutcomeUnknown},
		{fault.AccountListStale, locales.AccountListStale},
		{fault.AccountDefaultUnknown, locales.AccountDefaultUnknown},
		{fault.Stopping, locales.ErrorStopping},
		{fault.SessionLimit, locales.ErrorSessionLimit},
		{fault.SessionMissing, locales.ErrorSessionMissing},
		{fault.UnsupportedAgent, locales.ErrorUnsupportedAgent},
		{fault.ProjectDirectory, locales.ErrorProjectDirectory},
		{fault.ArgumentNUL, locales.ErrorArgumentNUL},
		{fault.InvalidArguments, locales.InvalidLaunchArgs},
		{fault.ExecutableMissing, locales.ErrorExecutableMissing},
		{fault.StopTimeout, locales.ErrorStopTimeout},
		{fault.StorageVersion, locales.ErrorStorageVersion},
		{fault.WorkspaceData, locales.ErrorWorkspaceData},
		{fault.ResumeReference, locales.ErrorResumeReference},
		{fault.CLIUnavailable, locales.ErrorCLIUnavailable},
		{fault.CLISchema, locales.ErrorCLISchema},
		{fault.CLIProtocol, locales.ErrorCLIProtocol},
		{fault.CLIContextChanged, locales.RemoteContextChanged},
		{fault.LoginOutcomeUnknown, locales.LoginOutcomeUnknown},
		{fault.LogoutOutcomeUnknown, locales.LogoutOutcomeUnknown},
		{fault.LocalSetupUnknown, locales.LocalSetupUnknown},
		{fault.LocalSetupIncomplete, locales.LocalSetupIncomplete},
		{fault.LocalSetupReview, locales.LocalSetupReviewRequired},
		{fault.WireGuardOutcomeUnknown, locales.WireGuardOutcomeUnknown},
		{fault.WireGuardExecutionFailed, locales.WireGuardExecutionFailed},
		{fault.WireGuardPreconditionFailed, locales.WireGuardPreconditionFailed},
		{fault.WireGuardPlanStale, locales.WireGuardTunnelPlanStale},
		{fault.WireGuardBridgeRequired, func() string { return locales.WireGuardTunnelPrerequisite("execution_bridge_required") }},
		{fault.WireGuardReview, locales.WireGuardReviewRequired},
		{fault.WireGuardOutputPath, locales.WireGuardAbsolutePathHelp},
		{fault.SessionCreateInputInvalid, locales.SessionCreateInputInvalid},
		{fault.SessionCreateOutcomeUnknown, locales.SessionCreateUnknown},
		{fault.SessionCreateConflict, locales.SessionCreateConflict},
		{fault.SessionCreateTargetChanged, locales.SessionCreateTargetChanged},
		{fault.SessionCreateDeleted, locales.SessionCreateDeleted},
		{fault.SessionStopInputInvalid, locales.ErrorCLIProtocol},
		{fault.SessionStopOutcomeUnknown, locales.RemoteStopUnknown},
		{fault.SessionDeleteInputInvalid, locales.ErrorCLIProtocol},
		{fault.SessionDeleteOutcomeUnknown, locales.RemoteDeleteUnknown},
		{fault.SessionDeleteConflict, locales.RemoteDeleteConflict},
		{fault.SessionDeleteBlocked, locales.RemoteDeleteFailed},
		{context.Canceled, locales.RemoteQueryCanceled},
		{context.DeadlineExceeded, locales.RemoteServerUnavailable},
		{os.ErrPermission, locales.ErrorPermission},
		{os.ErrNotExist, locales.ErrorNotFound},
	} {
		if errors.Is(err, entry.code) {
			return entry.text()
		}
	}
	return locales.ErrorUnknown(err.Error())
}

func Notice(err error) func() string { return func() string { return Error(err) } }
