package service

import (
	"context"
	"log/slog"
	"net/http"
	"testing"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/contexts/viewer"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mdm/android"
	android_mock "github.com/fleetdm/fleet/v4/server/mdm/android/mock"
	"github.com/fleetdm/fleet/v4/server/ptr"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/androidmanagement/v1"
	"google.golang.org/api/googleapi"
)

func TestEnterpriseVerificationPreservesEnrollment(t *testing.T) {
	for _, tc := range []struct {
		name        string
		enterprises []*androidmanagement.Enterprise
		listErr     error
		wantErr     bool
	}{
		{name: "existing enterprise", enterprises: []*androidmanagement.Enterprise{{Name: "enterprises/original"}}},
		{name: "empty project", wantErr: true},
		{name: "different provider or project", enterprises: []*androidmanagement.Enterprise{{Name: "enterprises/other"}}, wantErr: true},
		{name: "suffix is not the enterprise", enterprises: []*androidmanagement.Enterprise{{Name: "enterprises/not-original"}}, wantErr: true},
		{name: "nil enterprise entry", enterprises: []*androidmanagement.Enterprise{nil}, wantErr: true},
		{name: "unconfirmed not found", listErr: &googleapi.Error{Code: http.StatusNotFound, Message: "not found"}, wantErr: true},
		{name: "unbound proxy deletion confirmation", listErr: &googleapi.Error{Code: http.StatusNotFound, Message: "PROXY_VERIFIED_DELETED: enterprise no longer exists"}, wantErr: true},
		{name: "permission denied", listErr: &googleapi.Error{Code: http.StatusForbidden, Message: "forbidden"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := InitCommonDSMocks()
			ds.GetEnterpriseFunc = func(context.Context) (*android.Enterprise, error) {
				return &android.Enterprise{EnterpriseID: "original"}, nil
			}
			client := &android_mock.Client{}
			client.InitCommonMocks()
			client.EnterprisesListFunc = func(context.Context, string) ([]*androidmanagement.Enterprise, error) {
				return tc.enterprises, tc.listErr
			}
			svc, err := NewServiceWithClient(slog.New(slog.DiscardHandler), ds, client, "test-key", &ds.DataStore, noopNewActivity, config.AndroidAgentConfig{})
			require.NoError(t, err)
			ctx := viewer.NewContext(t.Context(), viewer.Viewer{User: &fleet.User{ID: 1, GlobalRole: ptr.String(fleet.RoleAdmin)}})

			err = svc.VerifyExistingEnterpriseIfAny(ctx)
			if tc.wantErr {
				require.Error(t, err)
				require.False(t, fleet.IsNotFound(err), "unverified absence must not allow a replacement signup")
				_, err = svc.EnterpriseSignup(ctx)
				require.Error(t, err)
				require.False(t, ds.CreateEnterpriseFuncInvoked)
				require.False(t, client.SignupURLsCreateFuncInvoked)
			} else {
				require.NoError(t, err)
			}
			require.False(t, client.EnterpriseDeleteFuncInvoked)
			require.False(t, ds.DeleteZeroTouchEnrollmentTokensFuncInvoked)
			require.False(t, ds.DeleteAllEnterprisesFuncInvoked)
			require.False(t, ds.SetAndroidEnabledAndConfiguredFuncInvoked)
			require.False(t, ds.BulkSetAndroidHostsUnenrolledFuncInvoked)
			require.False(t, ds.MarkAllPendingAndroidVPPInstallsAsFailedFuncInvoked)
		})
	}
}
