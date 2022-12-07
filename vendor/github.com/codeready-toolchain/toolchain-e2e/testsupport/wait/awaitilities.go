package wait

import "fmt"

func NewAwaitilities(hostAwait *HostAwaitility, memberAwaitilities ...*MemberAwaitility) Awaitilities {
	return Awaitilities{
		hostAwaitility:     hostAwait,
		memberAwaitilities: memberAwaitilities,
	}
}

type Awaitilities struct {
	hostAwaitility     *HostAwaitility
	memberAwaitilities []*MemberAwaitility
}

func (a Awaitilities) Host() *HostAwaitility {
	return a.hostAwaitility
}

func (a Awaitilities) Member1() *MemberAwaitility {
	return a.memberAwaitilities[0]
}

func (a Awaitilities) Member2() *MemberAwaitility {
	return a.memberAwaitilities[1]
}

func (a Awaitilities) Member(name string) (*MemberAwaitility, error) {
	for _, m := range a.memberAwaitilities {
		if m.ClusterName == name {
			return m, nil
		}
	}
	return nil, fmt.Errorf("could not find awaitility for member '%s'", name)
}

func (a Awaitilities) AllMembers() []*MemberAwaitility {
	return a.memberAwaitilities
}
