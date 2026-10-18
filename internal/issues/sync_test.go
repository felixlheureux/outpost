package issues

import "testing"

func TestValidateDependenciesReportsMissingAndCycle(t *testing.T) {
	issues := map[int]Requirements{
		1: {Settings: Settings{DependsOn: []int{2}}},
		2: {Settings: Settings{DependsOn: []int{1}}},
		3: {Settings: Settings{DependsOn: []int{9}}},
	}
	problems := ValidateDependencies(issues)
	if problems[1] != "dependency cycle in this repository" || problems[2] != "dependency cycle in this repository" || problems[3] == "" {
		t.Fatalf("dependency problems = %+v", problems)
	}
}
