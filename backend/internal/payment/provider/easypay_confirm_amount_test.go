package provider

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

type easyPayAmountTransport struct {
	money string
}

func (r easyPayAmountTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"code":1,"status":1,"money":%q}`, r.money))),
	}, nil
}

func TestEasyPayConfirmNotificationRequiresFiniteAmounts(t *testing.T) {
	for _, money := range []string{"NaN", "+Inf", "-Inf", "0", "-1", "12.34"} {
		t.Run(money, func(t *testing.T) {
			provider := easyPayPoCProvider()
			provider.httpClient = &http.Client{Transport: easyPayAmountTransport{money: money}}
			notification := confirmTestNotification()
			notification.Amount = 12.34
			err := provider.ConfirmNotification(context.Background(), notification)
			if (err == nil) != (money == "12.34") {
				t.Fatalf("confirmation for upstream amount %q: %v", money, err)
			}
		})
	}
	for _, amount := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -1} {
		provider := easyPayPoCProvider()
		provider.httpClient = &http.Client{Transport: easyPayAmountTransport{money: "12.34"}}
		notification := confirmTestNotification()
		notification.Amount = amount
		if err := provider.ConfirmNotification(context.Background(), notification); err == nil {
			t.Fatalf("accepted invalid notification amount %v", amount)
		}
	}
}
