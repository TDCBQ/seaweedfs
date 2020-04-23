package weed_server

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/chrislusf/seaweedfs/weed/pb/master_pb"
)

/*
How exclusive lock works?
-----------

Shell
------
When shell lock,
  * lease an admin token (lockTime, token)
  * start a goroutine to renew the admin token periodically
For later volume operations, send (lockTime, token) to volume servers for exclusive access
  * need to pause renewal a few seconds, to prevent race condition

When shell unlock
  * stop the renewal goroutine
  * sends a release lock request

Master
------
Master maintains:
  * randomNumber
  * lastLockTime
When master receives the lease/renew request from shell
  If lastLockTime still fresh {
    if is a renew and token is valid {
      // for renew
      generate the randomNumber => token
      return
    }
    refuse
    return
  } else {
    // for fresh lease request
    generate the randomNumber => token
    return
  }

When master receives the release lock request from shell
  set the lastLockTime to zero

When master receives the verfication request from volume servers
  return secret+lockTime == token && lockTime == lastLockTime

Volume
------
When receives (lockTime, token), ask the master whether this is valid


*/

const (
	LockDuration = 10 * time.Second
)

func (ms *MasterServer) LeaseAdminToken(ctx context.Context, req *master_pb.LeaseAdminTokenRequest) (*master_pb.LeaseAdminTokenResponse, error) {
	resp := &master_pb.LeaseAdminTokenResponse{}

	if ms.adminAccessLockTime.Add(LockDuration).After(time.Now()) {
		if req.PreviousToken != 0 && ms.isValidToken(time.Unix(0, req.PreviousLockTime), req.PreviousToken) {
			// for renew
			ts, token := ms.generateToken()
			resp.Token, resp.LockTsNs = token, ts.UnixNano()
			return resp, nil
		}
		// refuse since still locked
		return resp, fmt.Errorf("already locked")
	}
	// for fresh lease request
	ts, token := ms.generateToken()
	resp.Token, resp.LockTsNs = token, ts.UnixNano()
	return resp, nil
}

func (ms *MasterServer) isValidToken(ts time.Time, token int64) bool {
	return ms.adminAccessLockTime.Equal(ts) && ms.adminAccessSecret == token
}
func (ms *MasterServer) generateToken() (ts time.Time, token int64) {
	ms.adminAccessLockTime = time.Now()
	ms.adminAccessSecret = rand.Int63()
	return ms.adminAccessLockTime, ms.adminAccessSecret
}

func (ms *MasterServer) ReleaseAdminToken(ctx context.Context, req *master_pb.ReleaseAdminTokenRequest) (*master_pb.ReleaseAdminTokenResponse, error) {
	resp := &master_pb.ReleaseAdminTokenResponse{}
	if ms.isValidToken(time.Unix(0, req.PreviousLockTime), req.PreviousToken) {
		ms.adminAccessSecret = 0
	}
	return resp, nil
}

func (ms *MasterServer) VerifyAdminToken(ctx context.Context, req *master_pb.VerifyAdminTokenRequest) (*master_pb.VerifyAdminTokenResponse, error) {
	resp := &master_pb.VerifyAdminTokenResponse{}
	if ms.isValidToken(time.Unix(0, req.LockTime), req.Token) {
		resp.IsValid = true
	}
	return resp, nil
}
