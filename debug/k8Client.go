package debug

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	typesv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	k8sTimeout           = 30 * time.Second
	defaultNamespace     = "default"
	tridentNamespace     = "trident"
	QPS                  = 50
	burstTime            = 100
	TridentCSILabelKey   = "app"
	TridentCSILabelValue = "controller.csi.trident.netapp.io"
	TridentCSILabel      = TridentCSILabelKey + "=" + TridentCSILabelValue
)

var (
	KubeConfigPath       string
	CLIKubernetes        = "kubectl"
	client               *Clients
	artifactoryNamespace string
	artifactoryFolder    string
)

type Clients struct {
	RestConfig *rest.Config
	KubeClient *KubeClient
	Namespace  string
}

type KubeClient struct {
	clientset    kubernetes.Interface
	extClientset apiextension.Interface
	restConfig   *rest.Config
	namespace    string
	versionInfo  *version.Info
	cli          string
	timeout      time.Duration
}

func createK8sClient(
	masterURL, kubeConfigPath, overrideNamespace string,
) (*Clients, error) {
	var namespace string
	var restConfig *rest.Config
	var err error

	if kubeConfigPath != "" {
		if restConfig, err = clientcmd.BuildConfigFromFlags(masterURL, kubeConfigPath); err != nil {
			return nil, err
		}

		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.ExplicitPath = kubeConfigPath

		apiConfig, err := rules.Load()
		if err != nil {
			return nil, err
		} else if apiConfig.CurrentContext == "" {
			return nil, errors.New("current context is empty")
		}

		currentContext := apiConfig.Contexts[apiConfig.CurrentContext]
		if currentContext == nil {
			return nil, errors.New("current context is nil")
		}
		namespace = currentContext.Namespace

	} else {
		// c.cli config view --raw
		args := []string{"config", "view", "--raw"}

		out, err := exec.Command(CLIKubernetes, args...).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("%s; %v", string(out), err)
		}

		clientConfig, err := clientcmd.NewClientConfigFromBytes(out)
		if err != nil {
			return nil, err
		}

		restConfig, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}

		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return nil, err
		}
	}

	if namespace == "" {
		namespace = defaultNamespace
	}
	if overrideNamespace != "" {
		namespace = overrideNamespace
	}

	// Create the CLI-based Kubernetes client
	restConfig.QPS = QPS
	restConfig.Burst = burstTime
	k8sClient, err := NewKubeClient(restConfig, namespace, k8sTimeout)
	if err != nil {
		return nil, fmt.Errorf("could not initialize Kubernetes client; %v", err)
	}

	return &Clients{
		RestConfig: restConfig,
		KubeClient: k8sClient,
		Namespace:  namespace,
	}, nil
}

func NewKubeClient(config *rest.Config, namespace string, k8sTimeout time.Duration) (*KubeClient, error) {
	var versionInfo *version.Info
	if namespace == "" {
		return nil, errors.New("an empty namespace is not acceptable")
	}

	// Create core client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Create extension client
	extClientset, err := apiextension.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	versionInfo, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("couldn't retrieve API server's version: %v", err)
	}

	kubeClient := &KubeClient{
		clientset:    clientset,
		extClientset: extClientset,
		restConfig:   config,
		namespace:    namespace,
		versionInfo:  versionInfo,
		timeout:      k8sTimeout,
		cli:          "kubectl",
	}

	return kubeClient, nil
}

func discoverKubernetesCLI() error {
	_, err := exec.Command(CLIKubernetes, "version").Output()
	if GetExitCodeFromError(err) == ExitCodeSuccess {
		return nil
	}

	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Errorf("found the Kubernetes CLI, but it exited with error: %s",
			strings.TrimRight(string(ee.Stderr), "\n"))
	}

	return fmt.Errorf("could not find the Kubernetes CLI: %v", err)
}

func initClient() (err error) {
	client, err = createK8sClient("", KubeConfigPath, tridentNamespace)
	if err != nil {
		return err
	}

	return nil
}

func InitDebug(ctx context.Context, kubeConfigPath string, artifactoryNS string, artifactoryF string) (err error) {
	if kubeConfigPath != "" {
		KubeConfigPath = kubeConfigPath
	}

	if artifactoryNS != "" {
		artifactoryNamespace = artifactoryNS
	}

	if artifactoryF != "" {
		artifactoryFolder = artifactoryF
	}

	if err = discoverKubernetesCLI(); err != nil {
		return err
	}

	if err = initClient(); err != nil {
		return err
	}

	if err = getTridentDeployment(ctx); err != nil {
		return err
	}

	return nil
}

func (k *KubeClient) GetDeployment() typesv1.DeploymentInterface {
	deploymentSet := k.clientset.AppsV1().Deployments(k.namespace)
	return deploymentSet
}

// GetPodByLabel returns a pod object matching the specified label
func (k *KubeClient) GetPodByLabel(label string, allNamespaces bool) (*corev1.Pod, error) {
	pods, err := k.GetPodsByLabel(label, allNamespaces)
	if err != nil {
		return nil, err
	}

	if len(pods) == 1 {
		return &pods[0], nil
	} else if len(pods) > 1 {
		return nil, fmt.Errorf("multiple pods have the label %s", label)
	} else {
		return nil, fmt.Errorf("no pods have the label %s", label)
	}
}

// GetPodsByLabel returns all pod objects matching the specified label
func (k *KubeClient) GetPodsByLabel(label string, allNamespaces bool) ([]corev1.Pod, error) {
	listOptions, err := k.listOptionsFromLabel(label)
	if err != nil {
		return nil, err
	}

	namespace := k.namespace
	if allNamespaces {
		namespace = ""
	}

	podList, err := k.clientset.CoreV1().Pods(namespace).List(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

// listOptionsFromLabel accepts a label in the form "key=value" and returns a ListOptions value
// suitable for passing to the K8S API.
func (k *KubeClient) listOptionsFromLabel(label string) (metav1.ListOptions, error) {
	selector, err := k.getSelectorFromLabel(label)
	if err != nil {
		return metav1.ListOptions{}, err
	}

	return metav1.ListOptions{LabelSelector: selector}, nil
}

// getSelectorFromLabel accepts a label in the form "key=value" and returns a string in the
// correct form to pass to the K8S API as a LabelSelector.
func (k *KubeClient) getSelectorFromLabel(label string) (string, error) {
	selectorSet := make(labels.Set)

	if label != "" {
		labelParts := strings.Split(label, "=")
		if len(labelParts) != 2 {
			return "", fmt.Errorf("invalid label: %s", label)
		}
		selectorSet[labelParts[0]] = labelParts[1]
	}

	return selectorSet.String(), nil
}
