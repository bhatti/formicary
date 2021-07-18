package kubernetes

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"plexobject.com/formicary/ants/config"
)

func getKubeClient(config *config.AntConfig) (
	cli *kubernetes.Clientset,
	restConfig *restclient.Config,
	err error) {
	if config.Kubernetes.Host == "" {
		restConfig, err = guessClientConfig()
	} else {
		restConfig, err = getOutClusterClientConfig(&config.Kubernetes)
	}
	if err != nil {
		return nil, nil, err
	}
	cli, err = kubernetes.NewForConfig(restConfig)
	return
}

func getOutClusterClientConfig(
	config *config.KubernetesConfig) (*restclient.Config, error) {
	kubeConfig := &restclient.Config{
		Host:        config.Host,
		BearerToken: config.BearerToken,
		TLSClientConfig: restclient.TLSClientConfig{
			CAFile: config.CAFile,
		},
	}

	// certificate based auth
	if config.CertFile != "" {
		if config.KeyFile == "" || config.CAFile == "" {
			return nil, fmt.Errorf("ca file, cert file and key file must be specified when using file based auth")
		}

		kubeConfig.TLSClientConfig.CertFile = config.CertFile
		kubeConfig.TLSClientConfig.KeyFile = config.KeyFile
	} else if len(config.Username) > 0 {
		kubeConfig.Username = config.Username
		kubeConfig.Password = config.Password
	}

	//kubeConfig.Insecure = true
	return kubeConfig, nil
}

func guessClientConfig() (*restclient.Config, error) {
	// Try in cluster config first
	if inClusterCfg, err := restclient.InClusterConfig(); err == nil {
		return inClusterCfg, nil
	}

	// in cluster config failed. Reading default kubectl config
	return defaultKubectlConfig()
}

// See https://godoc.org/k8s.io/client-go/tools/clientcmd#ClientConfigLoadingRules
// https://godoc.org/k8s.io/client-go/tools/clientcmd/api#Config
func defaultKubectlConfig() (*restclient.Config, error) {
	load, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}

	return clientcmd.NewDefaultClientConfig(*load, &clientcmd.ConfigOverrides{}).ClientConfig()
}

func closeKubeClient(client *kubernetes.Clientset) bool {
	if client == nil {
		return false
	}
	rest, ok := client.CoreV1().RESTClient().(*restclient.RESTClient)
	if !ok || rest.Client == nil || rest.Client.Transport == nil {
		return false
	}
	if transport, ok := rest.Client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
		return true
	}
	return false
}