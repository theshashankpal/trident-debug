package debug

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var (
	tridentControllerDeploymentName = "trident-controller"
	tridentDeploymentMainContainer  = "trident-main"
)

// getTridentDeployment function is used to get the trident deployment object from the kubernetes cluster.
// It then modifies the deployment object to add the dlv debugger to the trident-main container.
// And deploy the updated deployment object to the kubernetes cluster.
func getTridentDeployment(ctx context.Context) (err error) {

	// Extracting the deploymentChan from the context
	deploymentChan := ctx.Value("deploymentChan").(chan int)

	deploymentSet := client.KubeClient.GetDeployment()
	tridentDeployment, err := deploymentSet.Get(context.TODO(), tridentControllerDeploymentName, metav1.GetOptions{})
	fmt.Printf("Found deployment %s in namespace %s\n", tridentDeployment.Name, tridentDeployment.Namespace)

	// Writing the deployment object to a yaml file, in copy_directory
	specYaml, err := yaml.Marshal(tridentDeployment.Spec)
	if err != nil {
		fmt.Println("An error occurred:", err)
		return
	}
	err = os.WriteFile("copy_directory/trident-controller-deployment.yaml", specYaml, 0644)
	if err != nil {
		fmt.Println("An error occurred during writing trident-controller-deployment onto a yaml:", err)
	}

	// Making a copy of the deployment object
	tridentDeploymentCopy := tridentDeployment.DeepCopy()
	for i, container := range tridentDeploymentCopy.Spec.Template.Spec.Containers {
		// If the container name matches the trident-main container
		if container.Name == tridentDeploymentMainContainer {
			// Get the container.
			tridentMainContainer := &tridentDeploymentCopy.Spec.Template.Spec.Containers[i]

			// Inserting `dlv` args at the beginning of existing args.
			argsCopy := tridentMainContainer.Args
			index := 0
			argsCopy = append(argsCopy[:index+1], argsCopy[index:]...)
			argsCopy[index] = "--"
			delveArgs := []string{"--listen=:40000",
				"--headless=true",
				"--continue",
				"--api-version=2",
				"--accept-multiclient",
				"exec",
			}
			args := append(delveArgs, tridentMainContainer.Command...)
			args = append(args, argsCopy...)
			tridentMainContainer.Args = args

			// Change the command to dlv.
			tridentMainContainer.Command = []string{"/dlv"}

			// Adding port 40000 to the container on which dlv is exposed.
			containerPorts := tridentMainContainer.Ports
			containerPorts = append(containerPorts, corev1.ContainerPort{
				ContainerPort: 40000,
				Protocol:      "TCP",
			})
			tridentMainContainer.Ports = containerPorts

			// Changing the image of the `trident-main` container
			// for ex: artifactory_namespace = pshashan and artifactory_folder = trident-debug
			// the image will be `docker.repo.eng.netapp.com/pshashan/trident-debug:latest`
			// if artifactory_folder is empty, the image will be `docker.repo.eng.netapp.com/pshashan/trident-debug:latest`
			if artifactoryFolder != "" {
				tridentMainContainer.Image = "docker.repo.eng.netapp.com" + "/" + artifactoryNamespace + "/" + artifactoryFolder + "/trident-debug:latest"
			} else {
				tridentMainContainer.Image = "docker.repo.eng.netapp.com" + "/" + artifactoryNamespace + "/trident-debug:latest"
			}
			tridentMainContainer.ImagePullPolicy = corev1.PullAlways

			// Adding SYS_PTRACE capability to the container
			tridentMainContainer.SecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"SYS_PTRACE"},
				},
				RunAsNonRoot: func(b bool) *bool { return &b }(false),
			}

			break
		}
	}

	_, err = deploymentSet.Update(context.TODO(), tridentDeploymentCopy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	fmt.Println("Sleeping for 10 seconds, so that cache is updated...")
	time.Sleep(10 * time.Second)

	// Checking if the deployment is updated successfully.
	fmt.Println("Checking if the deployment is updated successfully...")
	deploymentSet = client.KubeClient.GetDeployment()
	for {
		deployment, _ := deploymentSet.Get(context.TODO(), tridentControllerDeploymentName, metav1.GetOptions{})
		if deployment.Status.ReadyReplicas == deployment.Status.Replicas {
			break
		}
		fmt.Println("Waiting for the deployment to be updated...")
		time.Sleep(500 * time.Millisecond)
	}

	pod, err := client.KubeClient.GetPodByLabel(TridentCSILabel, false)
	if err != nil {
		return err
	}
	for {
		allContainersReady := true
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil {
				allContainersReady = false
				break
			}
		}
		if allContainersReady {
			fmt.Println("All containers in the pod are ready")
			break
		}
		fmt.Println("Waiting for all containers to be ready...")
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println("Deployment updated successfully.")

	//fmt.Println("Starting port-forwarding to the trident-main...")
	//err, _, stopChan := startPortForwarding(pod)
	//if err != nil {
	//	fmt.Println("An error occurred during port-forwarding:", err)
	//	deploymentChan <- 1 // Signaling that the deployment has been updated and containers are up and running.
	//	fmt.Println("Reverting the changes made to the deployment...")
	//	// Get the latest version of the deployment
	//	latestTridentDeployment, _ := deploymentSet.Get(context.TODO(), tridentControllerDeploymentName, metav1.GetOptions{})
	//
	//	// Reverting the changes made to the deployment.
	//	latestTridentDeployment.Spec = tridentDeployment.Spec
	//	// Updating the deployment with the original spec.
	//	_, _ = deploymentSet.Update(context.TODO(), latestTridentDeployment, metav1.UpdateOptions{})
	//	return err
	//}

	//fmt.Println("Port-forwarding is ready at the port 40000")

	deploymentChan <- 1 // Signaling that the deployment has been updated and containers are up and running.

	<-ctx.Done() // Waiting for the context to be canceled.

	//// Stopping the port-forwarding
	//fmt.Println("Stopping the port-forwarding...")
	//stopChan <- struct{}{}

	// Reverting the changes made to the deployment
	fmt.Println("Reverting the changes made to the deployment...")
	// Get the latest version of the deployment
	latestTridentDeployment, err := deploymentSet.Get(context.TODO(), tridentControllerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// Reverting the changes made to the deployment.
	latestTridentDeployment.Spec = tridentDeployment.Spec
	// Updating the deployment with the original spec.
	_, err = deploymentSet.Update(context.TODO(), latestTridentDeployment, metav1.UpdateOptions{})

	return err
}

// startPortForwarding function is used to start port forwarding to the trident-main container.
func startPortForwarding(pod *corev1.Pod) (error, chan struct{}, chan struct{}) {

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod not running: %s", pod.Name), nil, nil
	}

	// URL to the pod's portforward endpoint
	// e.g., http://localhost:8080/api/v1/namespaces/default/pods/pod-name/portforward

	url := client.KubeClient.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(client.RestConfig)
	if err != nil {
		return err, nil, nil
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)

	// Set local and remote ports
	localPort := "40000"
	remotePort := "40000"

	ports := []string{fmt.Sprintf("%s:%s", localPort, remotePort)}

	readyChan := make(chan struct{}, 1)
	stopChan := make(chan struct{}, 1)

	// Create a file to write the output
	outFile, err := os.Create("portforward_output.txt")
	if err != nil {
		return err, nil, nil
	}

	// Create a file to write the error
	errFile, err := os.Create("portforward_error.txt")
	if err != nil {
		return err, nil, nil
	}

	pf, err := portforward.New(dialer, ports, readyChan, stopChan, outFile, errFile)
	if err != nil {
		return err, nil, nil
	}

	// Start port forwarding in a separate goroutine
	go func() {
		err = pf.ForwardPorts()
		if err != nil {
			fmt.Println("An error occurred during port-forwarding:", err)
		}
	}()

	return nil, readyChan, stopChan
}
