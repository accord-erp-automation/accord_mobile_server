package core

import (
	"testing"

	"mobile_server/internal/erpnext"
)

func TestBuildCustomerDeliveryResultEventAccepted(t *testing.T) {
	item := erpnext.DeliveryNoteDraft{
		Name:         "MAT-DN-0001",
		Customer:     "CUST-001",
		CustomerName: "Comfi",
		ItemCode:     "ITEM-001",
		ItemName:     "Chers",
		Qty:          4,
		UOM:          "Nos",
		PostingDate:  "2026-03-14",
		DocStatus:    1,
		AccordFlowState:    "1",
		AccordCustomerState: "1",
	}

	record, ok := buildCustomerDeliveryResultEvent(item)
	if !ok {
		t.Fatalf("expected accepted result event")
	}
	if record.ID != customerDeliveryResultEventPrefix+"MAT-DN-0001" {
		t.Fatalf("unexpected event id: %q", record.ID)
	}
	if record.EventType != "customer_delivery_confirmed" {
		t.Fatalf("unexpected event type: %q", record.EventType)
	}
	if record.Highlight != "Customer mahsulotni qabul qildi" {
		t.Fatalf("unexpected highlight: %q", record.Highlight)
	}
	if record.Status != "accepted" {
		t.Fatalf("unexpected status: %q", record.Status)
	}
}

func TestBuildCustomerDeliveryResultEventRejected(t *testing.T) {
	item := erpnext.DeliveryNoteDraft{
		Name:         "MAT-DN-0002",
		Customer:     "CUST-001",
		CustomerName: "Comfi",
		ItemCode:     "ITEM-002",
		ItemName:     "Test",
		Qty:          7,
		UOM:          "Nos",
		PostingDate:  "2026-03-14",
		DocStatus:    1,
		AccordFlowState:    "1",
		AccordCustomerState: "2",
		AccordCustomerReason: "Qabul qilinmadi",
	}

	record, ok := buildCustomerDeliveryResultEvent(item)
	if !ok {
		t.Fatalf("expected rejected result event")
	}
	if record.EventType != "customer_delivery_rejected" {
		t.Fatalf("unexpected event type: %q", record.EventType)
	}
	if record.Highlight != "Customer mahsulotni rad etdi" {
		t.Fatalf("unexpected highlight: %q", record.Highlight)
	}
	if record.Note != "Customer rad etdi. Sabab: Qabul qilinmadi" {
		t.Fatalf("unexpected note: %q", record.Note)
	}
	if record.Status != "rejected" {
		t.Fatalf("unexpected status: %q", record.Status)
	}
}

func TestBuildCustomerDeliveryResultEventSkipsPending(t *testing.T) {
	item := erpnext.DeliveryNoteDraft{
		Name:         "MAT-DN-0003",
		Customer:     "CUST-001",
		CustomerName: "Comfi",
		ItemCode:     "ITEM-003",
		ItemName:     "Pending",
		Qty:          3,
		UOM:          "Nos",
		PostingDate:  "2026-03-14",
		DocStatus:    0,
	}
	if _, ok := buildCustomerDeliveryResultEvent(item); ok {
		t.Fatalf("pending delivery should not produce result event")
	}
}
