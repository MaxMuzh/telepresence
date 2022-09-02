package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/datawire/dlib/dlog"
	rpc "github.com/telepresenceio/telepresence/rpc/v2/manager"
)

type DNSRequest interface {
	GetName() string
	GetSession() *rpc.SessionInfo
}

type dnsAgentResponse[Q DNSRequest, R fmt.Stringer] interface {
	GetSession() *rpc.SessionInfo
	GetRequest() Q
	GetResponse() R
}

type lookupChannelFunc func(*agentSessionState) chan DNSRequest

// AgentsLookupHost will send the given request to all agents currently intercepted by the client identified with
// the clientSessionID, it will then wait for results to arrive, collect those results, and return them as a
// unique and sorted slice together with a count of how many agents that replied.
func (s *State) agentsLookup(ctx context.Context, clientSessionID string, request DNSRequest, lcf lookupChannelFunc) ([]fmt.Stringer, error) {
	iceptAgentIDs := s.getAgentsInterceptedByClient(clientSessionID)
	iceptCount := len(iceptAgentIDs)
	if iceptCount == 0 {
		return nil, nil
	}

	rsBuf := make(chan fmt.Stringer, iceptCount)

	agentTimeout, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(iceptCount)
	for _, agentSessionID := range iceptAgentIDs {
		go func(agentSessionID string) {
			responseID := request.GetSession().SessionId + ":" + request.GetName()
			defer func() {
				s.endLookup(agentSessionID, responseID)
				wg.Done()
			}()

			rsCh := s.startLookup(agentSessionID, responseID, request, lcf)
			if rsCh == nil {
				return
			}
			select {
			case <-agentTimeout.Done():
			case rs, ok := <-rsCh:
				if ok {
					dlog.Debugf(ctx, "Read lookup response from response channel")
					rsBuf <- rs
				}
			}
		}(agentSessionID)
	}
	wg.Wait() // wait for timeout or that all agents have responded
	bz := len(rsBuf)
	rs := make([]fmt.Stringer, bz)
	for i := 0; i < bz; i++ {
		rs[i] = <-rsBuf
	}
	dlog.Debugf(ctx, "agentsLookup returns %v", rs)
	return rs, nil
}

// PostLookupHostResponse receives lookup responses from an agent and places them in the channel
// that corresponds to the lookup request
func postLookupResponse[Q DNSRequest, R fmt.Stringer](ctx context.Context, s *State, response dnsAgentResponse[Q, R]) {
	request := response.GetRequest()
	responseID := request.GetSession().SessionId + ":" + request.GetName()
	var rch chan<- fmt.Stringer
	s.mu.RLock()
	if as, ok := s.sessions[response.GetSession().SessionId].(*agentSessionState); ok {
		dlog.Debugf(s.ctx, "Obtaining response channel for id %s", responseID)
		rch = as.lookupResponses[responseID]
	}
	s.mu.RUnlock()
	if rch != nil {
		dlog.Debugf(ctx, "Posting lookup response to response channel")
		rch <- response.GetResponse()
	} else {
		dlog.Debugf(ctx, "Posting lookup response but there was no channel")
	}
}

func (s *State) watchLookup(agentSessionID string, lcf lookupChannelFunc) <-chan DNSRequest {
	s.mu.RLock()
	ss, ok := s.sessions[agentSessionID]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return lcf(ss.(*agentSessionState))
}

func (s *State) startLookup(agentSessionID, responseID string, request DNSRequest, lcf lookupChannelFunc) <-chan fmt.Stringer {
	var (
		rch chan fmt.Stringer
		as  *agentSessionState
		ok  bool
	)
	s.mu.Lock()
	if as, ok = s.sessions[agentSessionID].(*agentSessionState); ok {
		if rch, ok = as.lookupResponses[responseID]; !ok {
			dlog.Debugf(s.ctx, "Creating response channel for id %s", responseID)
			rch = make(chan fmt.Stringer)
			as.lookupResponses[responseID] = rch
		}
	}
	s.mu.Unlock()
	if as != nil {
		// the as.lookups channel may be closed at this point, so guard for panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					close(rch)
				}
			}()
			lcf(as) <- request
		}()
	}
	return rch
}

func (s *State) endLookup(agentSessionID, responseID string) {
	s.mu.Lock()
	if as, ok := s.sessions[agentSessionID].(*agentSessionState); ok {
		if rch, ok := as.lookupResponses[responseID]; ok {
			dlog.Debugf(s.ctx, "Deleting response channel for id %s", responseID)
			delete(as.lookupResponses, responseID)
			close(rch)
		}
	}
	s.mu.Unlock()
}
