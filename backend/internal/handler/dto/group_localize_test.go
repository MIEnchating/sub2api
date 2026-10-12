package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func localizableGroup() *Group {
	return &Group{
		ID: 1, Name: "标准", Description: "号池",
		I18n: domain.GroupI18n{"en": {Name: "Standard", Description: "Pro + Max pool"}},
	}
}

func TestGroupLocalized_ReplacesTextAndDropsTranslations(t *testing.T) {
	g := localizableGroup().Localized("en")

	require.Equal(t, "Standard", g.Name)
	require.Equal(t, "Pro + Max pool", g.Description)

	raw, err := json.Marshal(g)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NotContains(t, decoded, "i18n", "user-facing output must not carry the translation table")
}

func TestGroupLocalized_KeepsOriginalWhenLanguageHasNoTranslation(t *testing.T) {
	g := localizableGroup().Localized("zh")

	require.Equal(t, "标准", g.Name)
	require.Equal(t, "号池", g.Description)
	require.Nil(t, g.I18n)
}

func TestLocalized_NilSafe(t *testing.T) {
	require.Nil(t, (*Group)(nil).Localized("en"))
	require.Nil(t, (*APIKey)(nil).Localized("en"))
	require.Nil(t, (*RedeemCode)(nil).Localized("en"))
	require.Nil(t, (*UsageLog)(nil).Localized("en"))
	require.Nil(t, (*UserSubscription)(nil).Localized("en"))

	// 未携带分组的记录原样返回
	require.Nil(t, (&APIKey{}).Localized("en").Group)
}

func TestLocalized_NestedGroups(t *testing.T) {
	key := (&APIKey{Group: localizableGroup()}).Localized("en")
	require.Equal(t, "Standard", key.Group.Name)

	code := (&RedeemCode{Group: localizableGroup()}).Localized("en")
	require.Equal(t, "Standard", code.Group.Name)

	sub := (&UserSubscription{Group: localizableGroup()}).Localized("en")
	require.Equal(t, "Standard", sub.Group.Name)

	// 使用记录除自身分组外，还带着 API Key 与订阅各自的分组
	log := (&UsageLog{
		Group:        localizableGroup(),
		APIKey:       &APIKey{Group: localizableGroup(), FallbackGroup: localizableGroup()},
		Subscription: &UserSubscription{Group: localizableGroup()},
	}).Localized("en")
	require.Equal(t, "Standard", log.Group.Name)
	require.Equal(t, "Standard", log.APIKey.Group.Name)
	require.Equal(t, "Standard", log.Subscription.Group.Name)
	require.Nil(t, log.APIKey.Group.I18n)
	require.Nil(t, log.Subscription.Group.I18n)
	require.Equal(t, "Standard", log.APIKey.FallbackGroup.Name)
	require.Nil(t, log.APIKey.FallbackGroup.I18n)
}

func TestAPIKeyLocalized_FallbackGroupPreservesAdminData(t *testing.T) {
	fallbackID := int64(2)
	fallback := &service.Group{
		ID: fallbackID, Name: "备用", Description: "备用号池",
		I18n: domain.GroupI18n{"en": {Name: "Fallback", Description: "Fallback pool"}},
	}
	source := &service.APIKey{FallbackGroupID: &fallbackID, FallbackGroup: fallback}
	key := APIKeyFromService(source).Localized("en")
	require.Equal(t, fallbackID, *key.FallbackGroupID)
	require.Equal(t, "Fallback", key.FallbackGroup.Name)
	require.Equal(t, "Fallback pool", key.FallbackGroup.Description)
	raw, err := json.Marshal(key)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NotContains(t, decoded["fallback_group"], "i18n")

	// User-facing localization must not change subsequent admin or other-locale responses.
	admin := APIKeyFromService(source)
	require.Equal(t, "备用", admin.FallbackGroup.Name)
	require.Equal(t, "Fallback", admin.FallbackGroup.I18n["en"].Name)
	require.Equal(t, "备用", APIKeyFromService(source).Localized("zh").FallbackGroup.Name)
}
