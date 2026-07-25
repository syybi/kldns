package providers

import (
	"testing"

	"kldns/pkg/dns"
)

var (
	_ dns.RecordValueManager = (*route53Provider)(nil)
	_ dns.RecordValueManager = (*googleProvider)(nil)
	_ dns.RecordValueManager = (*huaweiProvider)(nil)
)

func TestRecordValueHelpersPreserveRRSetSiblings(t *testing.T) {
	values := []string{"192.0.2.1", "192.0.2.2"}
	values, added := appendUniqueRecordValue(values, "192.0.2.3")
	if !added || len(values) != 3 {
		t.Fatalf("append result = %#v, added=%v", values, added)
	}
	values, replaced := replaceRecordValue(values, "192.0.2.2", "192.0.2.20")
	if !replaced || len(values) != 3 || values[0] != "192.0.2.1" || values[1] != "192.0.2.20" || values[2] != "192.0.2.3" {
		t.Fatalf("replace should preserve siblings: %#v", values)
	}
	values, removed := removeRecordValue(values, "192.0.2.20")
	if !removed || len(values) != 2 || values[0] != "192.0.2.1" || values[1] != "192.0.2.3" {
		t.Fatalf("remove should preserve siblings: %#v", values)
	}
}

func TestRecordValueHelpersRejectDuplicates(t *testing.T) {
	values := []string{"v=spf1 -all", "verification=abc"}
	if _, added := appendUniqueRecordValue(values, " v=spf1 -all "); added {
		t.Fatal("whitespace-equivalent duplicate should not be added")
	}
	if _, replaced := replaceRecordValue(values, "verification=abc", "v=spf1 -all"); replaced {
		t.Fatal("replacement should not collapse two RRset values into a duplicate")
	}
}
