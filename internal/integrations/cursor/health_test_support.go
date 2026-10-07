package cursor

import "context"

type HealthIdentityForTest = healthIdentity
type HealthResponseForTest = healthResponse

func ExpectedHealthForTest(dir, cwd string, internal bool) (HealthIdentityForTest, error) {
	return expectedHealth(dir, cwd, internal)
}
func HealthProofForTest(key, nonce string, id HealthIdentityForTest) string {
	return healthProof(key, nonce, id)
}
func RuntimeDigestForTest(dir string) (string, error)    { return runtimeDigest(dir) }
func HealthOKForTest(ctx context.Context, port int) bool { return healthOK(ctx, port) }
func SDKInstalledForTest(dir string) bool                { return sdkInstalled(dir) }
func VerifySidecarForTest(ctx context.Context, port int, id HealthIdentityForTest, key string) error {
	return verifySidecar(ctx, port, id, key)
}
func ManagedPIDForTest() int {
	mu.Lock()
	defer mu.Unlock()
	if running == nil || running.cmd == nil || running.cmd.Process == nil {
		return 0
	}
	return running.cmd.Process.Pid
}
