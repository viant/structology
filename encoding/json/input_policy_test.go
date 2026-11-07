package json

import (
	"github.com/viant/tagly/format/text"
	"reflect"
	"testing"
)

type inputPolicyCap struct {
	Count int `json:"count"`
}
type inputPolicyRow struct {
	Cap     *inputPolicyCap              `json:"frequencyCapping" format:"inputNames=frequencyCap,inputNamesPriority=canonical,inputNamesKinds=object|null,inputNamesCase=fold"`
	Flights []inputPolicyCap             `json:"campaignFlightCaps" format:"inputNames=flightConstraints,inputNamesPriority=canonical,inputNamesCase=fold,inputNamesExclude=FlightConstraints"`
	Has     *struct{ Cap, Flights bool } `json:"-" setMarker:"true"`
}

func TestInputPolicyNativeAndStandard(t *testing.T) {
	for _, standard := range []bool{false, true} {
		for _, tc := range []struct {
			raw           string
			want          int
			present, fail bool
		}{
			{`{}`, -1, false, false}, {`{"frequencyCap":3}`, -1, false, false}, {`{"frequencyCap":[]}`, -1, false, false},
			{`{"frequencyCap":null}`, -1, true, false}, {`{"FREQUENCYCAP":{"count":2}}`, 2, true, false},
			{`{"frequencyCapping":{"count":1},"frequencyCap":{"count":2}}`, 1, true, false},
			{`{"frequencyCap":{"count":2},"frequencyCapping":{"count":1}}`, 1, true, false},
			{`{"frequencyCapping":{"count":1},"frequencyCap":null}`, 1, true, false},
			{`{"frequencyCapping":null,"frequencyCap":{"count":2}}`, -1, true, false},
			{`{"frequencyCapping":3,"frequencyCap":{"count":2}}`, -1, false, true},
			{`{"frequencyCap":{"count":"bad"},"frequencyCapping":{"count":1}}`, 1, true, false},
			{`{"frequencyCap":{"count":"bad"}}`, -1, false, true},
			{`{"frequencyCap":[}`, -1, false, true},
		} {
			var r inputPolicyRow
			var err error
			if standard {
				err = UnmarshalStandard([]byte(tc.raw), &r, WithPresenceTracking(true))
			} else {
				err = Unmarshal([]byte(tc.raw), &r, WithPresenceTracking(true))
			}
			if tc.fail {
				if err == nil {
					t.Errorf("accepted standard=%v %s", standard, tc.raw)
				}
				continue
			}
			if err != nil {
				t.Fatalf("standard=%v %s: %v", standard, tc.raw, err)
			}
			got := -1
			if r.Cap != nil {
				got = r.Cap.Count
			}
			if got != tc.want || (r.Has != nil && r.Has.Cap) != tc.present {
				t.Errorf("standard=%v %s got%+v Has%+v", standard, tc.raw, r, r.Has)
			}
		}
		var r struct {
			Rows  []inputPolicyRow          `json:"rows"`
			Named map[string]inputPolicyRow `json:"named"`
		}
		raw := []byte(`{"rows":[{"flightConstraints":[{"count":2}],"FlightConstraints":[{"count":3}]}],"named":{"x":{"FLIGHTCONSTRAINTS":[{"count":4}]}}}`)
		var err error
		if standard {
			err = UnmarshalStandard(raw, &r)
		} else {
			err = Unmarshal(raw, &r)
		}
		if err != nil || !reflect.DeepEqual(r.Rows[0].Flights, []inputPolicyCap{{2}}) || !reflect.DeepEqual(r.Named["x"].Flights, []inputPolicyCap{{4}}) {
			t.Fatalf("nested standard=%v %+v %v", standard, r, err)
		}
	}
}

func TestInputPolicyDuplicateAndMalformedRules(t *testing.T) {
	var value inputPolicyRow
	if err := Unmarshal([]byte(`{"frequencyCap":3,"frequencyCap":4}`), &value, WithDuplicateKeyPolicy(ErrorOnDuplicate)); err == nil {
		t.Fatal("discarded aliases bypassed duplicate rejection")
	}
	for _, standard := range []bool{false, true} {
		for _, raw := range []string{`{"frequencyCapping":{"count":"bad"},"frequencyCapping":{"count":1}}`, `{"frequencyCap":tru,"frequencyCapping":{"count":1}}`} {
			var value inputPolicyRow
			var err error
			if standard {
				err = UnmarshalStandard([]byte(raw), &value)
			} else {
				err = Unmarshal([]byte(raw), &value)
			}
			if err == nil {
				t.Fatalf("standard=%v accepted %s", standard, raw)
			}
		}
	}
}

func TestInputPolicyFormattedCanonicalPriority(t *testing.T) {
	var value struct {
		FrequencyCapping *inputPolicyCap `format:"inputNames=frequencyCap,inputNamesPriority=canonical"`
	}
	if err := Unmarshal([]byte(`{"frequency_capping":{"count":1},"frequencyCap":{"count":2}}`), &value, WithCaseFormat(text.CaseFormatLowerUnderscore)); err != nil {
		t.Fatal(err)
	}
	if value.FrequencyCapping == nil || value.FrequencyCapping.Count != 1 {
		t.Fatalf("formatted canonical lost: %+v", value)
	}
}

func TestInputAliasPriorityDecoders(t *testing.T) {
	type row struct {
		ID  *int               `json:"flightId" format:"inputNames=CampaignFlightId,inputNamesPriority=alias,inputNamesCase=fold"`
		Has *struct{ ID bool } `json:"-" setMarker:"true"`
	}
	for _, standard := range []bool{false, true} {
		for _, tc := range []struct {
			raw           string
			want          int
			present, fail bool
		}{
			{`{}`, -1, false, false}, {`{"flightId":1}`, 1, true, false},
			{`{"CampaignFlightId":2,"flightId":1}`, 2, true, false}, {`{"flightId":1,"campaignflightid":2}`, 2, true, false},
			{`{"CampaignFlightId":null,"flightId":1}`, -1, true, false}, {`{"flightId":null,"CampaignFlightId":2}`, 2, true, false},
			{`{"CampaignFlightId":2,"flightId":"discarded"}`, 2, true, false}, {`{"flightId":"discarded","CampaignFlightId":2}`, 2, true, false},
			{`{"CampaignFlightId":"bad","flightId":1}`, -1, false, true}, {`{"CampaignFlightId":2,"flightId":[}`, -1, false, true},
			{`{"CampaignFlightId":2,"flightId":1,"CAMPAIGNFLIGHTID":3}`, 3, true, false},
			{`{"CampaignFlightId":"bad","CampaignFlightId":2}`, -1, false, true},
		} {
			var r row
			var err error
			if standard {
				err = UnmarshalStandard([]byte(tc.raw), &r, WithPresenceTracking(true))
			} else {
				err = Unmarshal([]byte(tc.raw), &r, WithPresenceTracking(true))
			}
			if tc.fail {
				if err == nil {
					t.Errorf("accepted %s", tc.raw)
				}
				continue
			}
			if err != nil {
				t.Fatalf("standard=%v %s: %v", standard, tc.raw, err)
			}
			got := -1
			if r.ID != nil {
				got = *r.ID
			}
			if got != tc.want || (r.Has != nil && r.Has.ID) != tc.present {
				t.Errorf("%s got%v has%+v", tc.raw, got, r.Has)
			}
		}
	}
}

func TestInputAliasPriorityNestedAndStrict(t *testing.T) {
	type row struct {
		ID int `json:"flightId" format:"inputNames=CampaignFlightId,inputNamesPriority=alias,inputNamesCase=fold"`
	}
	var r row
	for _, raw := range []string{`{"flightId":1,"flightId":2,"CampaignFlightId":3}`, `{"CampaignFlightId":1,"CampaignFlightId":2}`} {
		if err := Unmarshal([]byte(raw), &r, WithDuplicateKeyPolicy(ErrorOnDuplicate)); err == nil {
			t.Fatalf("duplicate accepted %s", raw)
		}
	}
	var nested struct {
		Rows  []row          `json:"rows"`
		Named map[string]row `json:"named"`
	}
	raw := []byte(`{"rows":[{"CampaignFlightId":2,"flightId":"ignored"}],"named":{"x":{"flightId":"ignored","CampaignFlightId":3}}}`)
	if err := Unmarshal(raw, &nested); err != nil {
		t.Fatal(err)
	}
	if nested.Rows[0].ID != 2 || nested.Named["x"].ID != 3 {
		t.Fatal(nested)
	}
	encoded, err := Marshal(nested.Rows[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"flightId":2}` {
		t.Fatal(string(encoded))
	}
}
