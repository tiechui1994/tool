package rules

import "fmt"

func ParseRule(tp, payload, target string, params []string) (rule Rule, parseErr error)  {
	switch tp {
	case RuleDomain:
		rule = NewDomain(payload, target)
	case RuleDomainKeyword:
		rule = NewDomainKeyword(payload, target)
	case RuleDomainSuffix:
		rule = NewDomainSuffix(payload, target)
		parseErr = nil
	case RuleIPCIDR:
		rule, parseErr = NewIPCIDR(payload, target)
	case RuleSrcPort:
		rule, parseErr = NewPort(payload, target, RuleSrcPort)
	case RuleDstPort:
		rule, parseErr = NewPort(payload, target, RuleDstPort)
	case RuleMatch:
		rule = NewMatch(target)
	default:
		parseErr = fmt.Errorf("unsupported rule type %s", tp)
	}
	if parseErr != nil {
		return nil, parseErr
	}

	return rule, nil
}