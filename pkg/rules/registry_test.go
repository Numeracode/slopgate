package rules

import "testing"

func TestDefault_RegistersAllV001Rules(t *testing.T) {
	r := Default()
	want := map[string]bool{
		"SLP001": false,
		"SLP002": false,
		"SLP003": false,
		"SLP005": false,
		"SLP006": false,
		"SLP007": false,
		"SLP008": false,
		"SLP009": false,
		"SLP010": false,
		"SLP011": false,
		"SLP012": false,
		"SLP013": false,
		"SLP014": false,
		"SLP015": false,
		"SLP016": false,
		"SLP017": false,
		"SLP018": false,
		"SLP019": false,
		"SLP020": false,
		"SLP021": false,
		"SLP022": false,
		"SLP023": false,
		"SLP024": false,
		"SLP025": false,
		"SLP026": false,
		"SLP027": false,
		"SLP030": false,
		"SLP031": false,
		"SLP032": false,
		"SLP033": false,
		"SLP034": false,
		"SLP035": false,
		"SLP036": false,
		"SLP037": false,
		"SLP038": false,
		"SLP039": false,
		"SLP040": false,
		"SLP041": false,
		"SLP042": false,
		"SLP043": false,
		"SLP044": false,
		"SLP045": false,
		"SLP046": false,
		"SLP047": false,
		"SLP048": false,
		"SLP049": false,
		"SLP050": false,
		"SLP051": false,
		"SLP052": false,
		"SLP053": false,
		"SLP054": false,
		"SLP055": false,
		"SLP056": false,
		"SLP057": false,
		"SLP058": false,
		"SLP059": false,
		"SLP060": false,
		"SLP061": false,
		"SLP062": false,
		"SLP063": false,
		"SLP064": false,
		"SLP065": false,
		"SLP066": false,
		"SLP067": false,
		"SLP068": false,
		"SLP069": false,
		"SLP070": false,
		"SLP081": false,
		"SLP082": false,
		"SLP083": false,
		"SLP084": false,
		"SLP085": false,
		"SLP086": false,
		"SLP087": false,
		"SLP088": false,
		"SLP089": false,
		"SLP090": false,
		"SLP091": false,
		"SLP092": false,
		"SLP093": false,
		"SLP094": false,
		"SLP095": false,
		"SLP096": false,
		"SLP097": false,
		"SLP098": false,
		"SLP099": false,
		"SLP100": false,
		"SLP101": false,
		"SLP102": false,
		"SLP103": false,
		"SLP104": false,
		"SLP106": false,
		"SLP107": false,
		"SLP108": false,
		"SLP109": false,
		"SLP110": false,
		"SLP111": false,
		"SLP112": false,
		"SLP113": false,
		"SLP114": false,
		"SLP115": false,
		"SLP116": false,
		"SLP117": false,
		"SLP118": false,
		"SLP119": false,
		"SLP120": false,
		"SLP121": false,
		"SLP122": false,
		"SLP123": false,
		"SLP124": false,
		"SLP125": false,
		"SLP126": false,
		"SLP127": false,
		"SLP128": false,
		"SLP129": false,
		"SLP130": false,
		"SLP131": false,
		"SLP132": false,
		"SLP133": false,
		"SLP134": false,
		"SLP135": false,
		"SLP136": false,
		"SLP137": false,
		"SLP138": false,
		"SLP139": false,
		"SLP140": false,
		"SLP141": false,
		"SLP142": false,
		"SLP157": false,
		"SLP158": false,
		"SLP159": false,
		"SLP203": false,
		"SLP204": false,
		"SLP205": false,
		"SLP207": false,
		"SLP208": false,
		"SLP209": false,
	}
	for _, rule := range r.All() {
		if _, ok := want[rule.ID()]; ok {
			want[rule.ID()] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Errorf("rule %s not registered in Default()", id)
		}
	}
}

func TestDefault_NoExtraRules(t *testing.T) {
	r := Default()

	// Includes SLP202-SLP205, SLP207, the P3 rules SLP151, SLP152, new precision rules SLP155-159, SLP035 split rules SLP160-162, reviewer gap rules SLP210-214, reviewer gap closure v2 rules SLP215-218, and reviewer gap closure v3 rules SLP219-222.
	wantCount := 164
	if got := len(r.All()); got != wantCount {
		t.Errorf("Default registry has %d rules, want %d", got, wantCount)
	}

	ruleIDs := make(map[string]bool)
	for _, rule := range r.All() {
		ruleIDs[rule.ID()] = true
	}

	newRules := []string{
		"SLP081", "SLP082", "SLP083", "SLP084", "SLP085",
		"SLP086", "SLP087", "SLP088", "SLP089", "SLP090",
		"SLP091", "SLP092", "SLP093", "SLP094", "SLP095",
		"SLP096", "SLP097", "SLP098", "SLP099", "SLP100",
		"SLP101", "SLP102", "SLP103", "SLP104",
		"SLP106", "SLP107", "SLP108", "SLP109", "SLP110",
		"SLP111", "SLP112",
		"SLP113", "SLP114", "SLP115", "SLP116", "SLP117",
		"SLP118", "SLP119", "SLP120",
		"SLP121", "SLP122", "SLP123", "SLP124", "SLP125",
		"SLP126", "SLP127",
		"SLP128", "SLP129", "SLP130", "SLP131", "SLP132",
		"SLP133", "SLP134", "SLP135", "SLP136", "SLP137",
		"SLP138", "SLP139", "SLP140", "SLP141", "SLP142",
		"SLP143", "SLP144", "SLP145", "SLP146", "SLP147", "SLP148",
		"SLP151", "SLP152",
		"SLP155", "SLP156",
		"SLP157", "SLP158", "SLP159",
		"SLP160", "SLP161", "SLP162",
		"SLP210", "SLP211", "SLP212", "SLP213", "SLP214",
		"SLP202",
		"SLP203",
		"SLP204",
		"SLP205",
		"SLP207",
		"SLP208",
		"SLP209",
	}
	for _, id := range newRules {
		if !ruleIDs[id] {
			t.Errorf("new rule %s not registered in Default()", id)
		}
	}
}
