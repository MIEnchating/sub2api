//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_CreateGroup_ImageCountBillingPlatformGate(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformComposite, PlatformGemini, PlatformAntigravity, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			repo := &groupRepoStubForAdmin{}
			svc := &adminServiceImpl{groupRepo: repo}
			group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
				Name: "image billing", Platform: platform, RateMultiplier: 1,
				AllowImageGeneration: true, ImageCountBilling: true,
			})
			require.NoError(t, err)
			require.Equal(t, platform == PlatformOpenAI || platform == PlatformComposite, group.ImageCountBilling)
			require.Equal(t, group.ImageCountBilling, repo.created.ImageCountBilling)
		})
	}
}

func TestAdminService_UpdateGroup_ImageCountBillingAndCacheInvalidation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input UpdateGroupInput
		want  bool
	}{
		{name: "omitted preserves enabled", want: true},
		{name: "explicit disable", input: UpdateGroupInput{ImageCountBilling: boolPtrForImageBillingTest(false)}},
		{name: "image permission disabled", input: UpdateGroupInput{AllowImageGeneration: boolPtrForImageBillingTest(false)}},
		{name: "unsupported platform", input: UpdateGroupInput{Platform: PlatformGemini}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &groupRepoStubForAdmin{getByID: &Group{
				ID: 9, Name: "images", Platform: PlatformOpenAI, Status: StatusActive,
				AllowImageGeneration: true, ImageCountBilling: true, UserConcurrencyLimit: 4,
			}}
			invalidator := &authCacheInvalidatorStub{}
			svc := &adminServiceImpl{groupRepo: repo, authCacheInvalidator: invalidator}
			group, err := svc.UpdateGroup(context.Background(), 9, &tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, group.ImageCountBilling)
			require.Equal(t, 4, group.UserConcurrencyLimit)
			require.Equal(t, []int64{9}, invalidator.groupIDs)
		})
	}
}
