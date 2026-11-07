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
