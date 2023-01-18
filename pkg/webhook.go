package pkg

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretName  = "monitoring-operator-secret-cert"
	webhookName = "monitoring-operator-mutating-webhook-config"
)

func (w *WebhookTls) RunWebHookTls() error {

	var keyPair *KeyPairArtifacts
	//检查secret存不存在，存在不创建，不存在创建
	keyPair, secretExist, err := w.checkTls()
	if err != nil {
		return err
	}

	if !secretExist && SelfSignedCa {
		keyPair, err = w.createCertPEM()
		if err != nil {
			return err
		}

		err = w.createSecret(keyPair.CertPEM, keyPair.KeyPEM)
		if err != nil {
			return err
		}

	}
	if !secretExist && !SelfSignedCa {
		return fmt.Errorf("secret %v doesn't exist", secretName)
	}
	err = w.createTls(keyPair.CertPEM, keyPair.KeyPEM)
	if err != nil {
		return err
	}

	if SelfSignedCa {
		err = w.updateCaBundle(keyPair.CertPEM)
		if err != nil {
			return err
		}
	}

	return nil
}

//检查secret是否存在
func (w *WebhookTls) checkTls() (*KeyPairArtifacts, bool, error) {
	secret, err := w.ClientSet.CoreV1().Secrets(w.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, false, nil
	}

	if err != nil {
		return nil, true, err
	}

	cert := secret.Data[certName]
	key := secret.Data[keyName]
	return &KeyPairArtifacts{CertPEM: cert, KeyPEM: key}, true, nil
}

//创建secret
func (w *WebhookTls) createSecret(cert []byte, key []byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: w.Namespace,
			Name:      secretName,
		},
		Data: map[string][]byte{
			certName: cert,
			keyName:  key,
		},
	}
	_, err := w.ClientSet.CoreV1().Secrets(w.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		_, err = w.ClientSet.CoreV1().Secrets(w.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}

	{
		_, err = w.ClientSet.CoreV1().Secrets(w.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

//更新webhookconfig caBundle字段
func (w *WebhookTls) updateCaBundle(cert []byte) error {
	MutatingWebhook, err := w.ClientSet.AdmissionregistrationV1().MutatingWebhookConfigurations().
		Get(context.Background(), webhookName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i := range MutatingWebhook.Webhooks {
		MutatingWebhook.Webhooks[i].ClientConfig.CABundle = cert
	}

	_, err = w.ClientSet.AdmissionregistrationV1().MutatingWebhookConfigurations().
		Update(context.Background(), MutatingWebhook, metav1.UpdateOptions{})

	if err != nil {
		return err
	}

	return nil
}

//创建webhook服务tls证书
func (w *WebhookTls) createTls(cert []byte, key []byte) error {
	certDir := w.CertDir
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		if err := os.MkdirAll(certDir, 0700); err != nil {
			return err
		}
	}

	if err := ioutil.WriteFile(path.Join(certDir, certName), cert, 0600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(path.Join(certDir, keyName), key, 0600); err != nil {
		return err
	}

	return nil
}
