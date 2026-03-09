package handler

import (
	"net/http"
	"time"

	"github.com/helix-acme-corp-demo/logpipe"

	helixpay "github.com/helix-acme-corp-demo/helix-pay-go"
	"github.com/helix-acme-corp-demo/helix-pay-go/charges"
	"github.com/helix-acme-corp-demo/helix-pay-go/webhooks"

	"github.com/helix-acme-corp-demo/billing-service/internal/store"
)

// NewHelixPayWebhookHandler returns an http.Handler that verifies and dispatches
// inbound HelixPay webhook events, updating invoice state accordingly.
//
// Handled events:
//   - charge.settled  → invoice status = "paid", paid_at = now
//   - charge.declined → invoice status = "draft" (allows retry)
//   - charge.voided   → invoice status = "draft" (allows retry)
func NewHelixPayWebhookHandler(s *store.Store, c *helixpay.Client, l logpipe.Logger) http.Handler {
	listener := webhooks.NewListener(c.Webhooks)

	listener.On(webhooks.ChargeSettled, func(event webhooks.Event) error {
		var charge charges.Charge
		if err := event.DataAs(&charge); err != nil {
			l.Error("helixpay webhook: failed to parse charge.settled payload",
				logpipe.String("event_id", event.ID),
				logpipe.String("error", err.Error()),
			)
			return err
		}

		invoice, ok := s.FindInvoiceByChargeID(charge.ID)
		if !ok {
			l.Error("helixpay webhook: invoice not found for charge",
				logpipe.String("event_type", "charge.settled"),
				logpipe.String("charge_id", charge.ID),
			)
			// Return nil so HelixPay gets a 200 — retrying won't help.
			return nil
		}

		now := time.Now().UTC()
		invoice.Status = "paid"
		invoice.PaidAt = &now
		s.SaveInvoice(invoice)

		l.Info("helixpay webhook: invoice marked paid",
			logpipe.String("invoice_id", invoice.ID),
			logpipe.String("charge_id", charge.ID),
		)
		return nil
	})

	listener.On(webhooks.ChargeDeclined, func(event webhooks.Event) error {
		var charge charges.Charge
		if err := event.DataAs(&charge); err != nil {
			l.Error("helixpay webhook: failed to parse charge.declined payload",
				logpipe.String("event_id", event.ID),
				logpipe.String("error", err.Error()),
			)
			return err
		}

		invoice, ok := s.FindInvoiceByChargeID(charge.ID)
		if !ok {
			l.Error("helixpay webhook: invoice not found for charge",
				logpipe.String("event_type", "charge.declined"),
				logpipe.String("charge_id", charge.ID),
			)
			return nil
		}

		invoice.Status = "draft"
		s.SaveInvoice(invoice)

		l.Info("helixpay webhook: charge declined, invoice reset to draft",
			logpipe.String("invoice_id", invoice.ID),
			logpipe.String("charge_id", charge.ID),
		)
		return nil
	})

	listener.On(webhooks.ChargeVoided, func(event webhooks.Event) error {
		var charge charges.Charge
		if err := event.DataAs(&charge); err != nil {
			l.Error("helixpay webhook: failed to parse charge.voided payload",
				logpipe.String("event_id", event.ID),
				logpipe.String("error", err.Error()),
			)
			return err
		}

		invoice, ok := s.FindInvoiceByChargeID(charge.ID)
		if !ok {
			l.Error("helixpay webhook: invoice not found for charge",
				logpipe.String("event_type", "charge.voided"),
				logpipe.String("charge_id", charge.ID),
			)
			return nil
		}

		invoice.Status = "draft"
		s.SaveInvoice(invoice)

		l.Info("helixpay webhook: charge voided, invoice reset to draft",
			logpipe.String("invoice_id", invoice.ID),
			logpipe.String("charge_id", charge.ID),
		)
		return nil
	})

	return listener
}
