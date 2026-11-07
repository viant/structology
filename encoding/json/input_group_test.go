package json

import "testing"

type groupDay struct {
	Canonical *int16                                  `json:"day_of_week" format:"inputGroup=weekday,inputPriority=0,inputNames=dayOfWeek,inputNamesPriority=alias"`
	Upper     *int                                    `json:"Weekday" format:"inputGroup=weekday,inputPriority=1,inputCase=exact"`
	Lower     *int16                                  `json:"weekday" format:"inputGroup=weekday,inputPriority=2,inputCase=exact"`
	Has       *struct{ Canonical, Upper, Lower bool } `json:"-" setMarker:"true"`
}

func TestInputGroupNativeStandard(t *testing.T) {
	for _, standard := range []bool{false, true} {
		for _, tc := range []struct {
			raw   string
			owner int
			fail  bool
		}{
			{`{}`, 0, false}, {`{"WEEKDAY":2}`, 0, false}, {`{"Weekday":2}`, 1, false}, {`{"weekday":2}`, 2, false},
			{`{"Weekday":"bad","weekday":null}`, 2, false}, {`{"weekday":null,"Weekday":"bad"}`, 2, false},
			{`{"day_of_week":"bad","Weekday":32768}`, 1, false}, {`{"Weekday":32768,"day_of_week":"bad"}`, 1, false},
			{`{"weekday":32768}`, 0, true}, {`{"Weekday":"bad","Weekday":2}`, 0, true},
			{`{"day_of_week":1}`, 3, false}, {`{"day_of_week":"bad","dayOfWeek":2}`, 3, false},
		} {
			var got groupDay
			var err error
			if standard {
				err = UnmarshalStandard([]byte(tc.raw), &got, WithPresenceTracking(true))
			} else {
				err = Unmarshal([]byte(tc.raw), &got, WithPresenceTracking(true), WithNumberPolicy(ExactNumbers))
			}
			if tc.fail {
				if err == nil {
					t.Errorf("accepted standard=%v %s", standard, tc.raw)
				}
				continue
			}
			if err != nil {
				t.Errorf("standard=%v %s: %v", standard, tc.raw, err)
				continue
			}
			upper, lower, canonical := false, false, false
			if got.Has != nil {
				upper, lower, canonical = got.Has.Upper, got.Has.Lower, got.Has.Canonical
			}
			if upper != (tc.owner == 1) || lower != (tc.owner == 2) || canonical != (tc.owner == 3) {
				t.Errorf("standard=%v %s Has=%+v", standard, tc.raw, got.Has)
			}
			if tc.owner != 1 && got.Upper != nil || tc.owner != 2 && got.Lower != nil || tc.owner != 3 && got.Canonical != nil {
				t.Errorf("losing value assigned: %+v", got)
			}
		}
	}
}

func TestInputGroupIgnoredSourceCannotWin(t *testing.T) {
	type row struct {
		Value  *int `json:"value" format:"inputGroup=x,inputPriority=0"`
		Hidden *int `json:"-" format:"inputGroup=x,inputPriority=100"`
	}
	for _, standard := range []bool{false, true} {
		var got row
		var err error
		if standard {
			err = UnmarshalStandard([]byte(`{"value":2,"Hidden":null,"-":null}`), &got)
		} else {
			err = Unmarshal([]byte(`{"value":2,"Hidden":null,"-":null}`), &got)
		}
		if err != nil || got.Value == nil || *got.Value != 2 || got.Hidden != nil {
			t.Fatalf("standard=%v got%+v err%v", standard, got, err)
		}
	}
}

func TestInputCanonicalVariantExclusion(t *testing.T) {
	type row struct {
		Status int `json:"Status" format:"inputNames=status,inputNamesExclude=status"`
	}
	for _, standard := range []bool{false, true} {
		var got row
		var err error
		if standard {
			err = UnmarshalStandard([]byte(`{"status":"ignored","STATUS":3}`), &got)
		} else {
			err = Unmarshal([]byte(`{"status":"ignored","STATUS":3}`), &got)
		}
		if err != nil || got.Status != 3 {
			t.Fatalf("standard%v got%+v err%v", standard, got, err)
		}
	}
}
