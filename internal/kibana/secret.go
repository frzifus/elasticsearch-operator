package kibana

import (
	"context"
	"sort"

	"github.com/ViaQ/logerr/kverrors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/elasticsearch-operator/internal/manifests/secret"
	"github.com/openshift/elasticsearch-operator/internal/utils"
	core "k8s.io/api/core/v1"
)

var secretCertificates = map[string]map[string]string{
	"kibana": {
		"ca":   "ca.crt",
		"key":  "system.logging.kibana.key",
		"cert": "system.logging.kibana.crt",
	},
	"kibana-proxy": {
		"server-key":     "kibana-internal.key",
		"server-cert":    "kibana-internal.crt",
		"session-secret": "kibana-session-secret",
	},
}

// readSecrets reads all of the secrets it can find within secretCertificates
// if any of the secrets are not found then they are ignored
func (clusterRequest *KibanaRequest) readSecrets() error {
	for secretName, certMap := range secretCertificates {
		err := clusterRequest.extractCertificates(secretName, certMap)
		if err != nil {
			return kverrors.Wrap(err, "failed to extract secret",
				"secret_name", secretName)
		}
	}

	return nil
}

func (clusterRequest *KibanaRequest) extractCertificates(secretName string, certs map[string]string) error {
	for secretKey, certPath := range certs {
		err := clusterRequest.extractSecretToFile(secretName, secretKey, certPath)
		if err != nil {
			return kverrors.Wrap(err, "failed to extract cert",
				"key", secretKey,
				"cert_path", certPath,
			)
		}
	}

	return nil
}

func (clusterRequest *KibanaRequest) extractSecretToFile(secretName string, key string, toFile string) (err error) {
	objKey := client.ObjectKey{Name: secretName, Namespace: clusterRequest.cluster.Namespace}
	sec, err := secret.Get(context.TODO(), clusterRequest.client, objKey)
	if err != nil {
		if apierrors.IsNotFound(kverrors.Root(err)) {
			return err
		}
		return kverrors.Wrap(err, "unable to extract secret to file",
			"cluster", clusterRequest.cluster.Name,
			"namespace", clusterRequest.cluster.Namespace,
		)
	}

	value, ok := sec.Data[key]

	// check to see if the map value exists
	if !ok {
		clusterRequest.log.Error(nil, "no secret data found", "key", key)
		return nil
	}

	return utils.WriteToWorkingDirFile(toFile, value)
}

func calcSecretHashValue(secret *core.Secret) (string, error) {
	hashValue := ""
	var err error

	if secret == nil {
		return hashValue, nil
	}

	var hashKeys []string
	var rawbytes []byte

	// we just want the keys here to sort them for consistently calculated hashes
	for key := range secret.Data {
		hashKeys = append(hashKeys, key)
	}

	sort.Strings(hashKeys)

	for _, key := range hashKeys {
		rawbytes = append(rawbytes, secret.Data[key]...)
	}

	hashValue, err = utils.CalculateMD5Hash(string(rawbytes))
	if err != nil {
		return "", err
	}

	return hashValue, nil
}
