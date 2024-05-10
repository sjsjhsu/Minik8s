package internal

import (
	"context"
	"minik8s/apiobjects"
	"minik8s/global"
	"minik8s/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cri "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func getContext() context.Context {
	return context.Background()
}

func getCriGrpcClient() (conn *grpc.ClientConn, err error) {
	// Create a gRPC client connection
	conn, err = grpc.Dial("unix:///run/containerd/containerd.sock", grpc.WithTransportCredentials(insecure.NewCredentials()))
	return
}

func getRuntimeServiceClient() (runtimeServiceClient cri.RuntimeServiceClient, err error) {
	// Create a gRPC client connection
	conn, err := getCriGrpcClient();
	if err != nil {
		return
	}
	// Create the runtime service client using the gRPC client connection
	runtimeServiceClient = cri.NewRuntimeServiceClient(conn)
	return
}

func getImageServiceClient() (imageSeviceClient cri.ImageServiceClient, err error) {
	// Create a gRPC client connection
	conn, err := getCriGrpcClient();
	if err != nil {
		return
	}
	// Create the runtime service client using the gRPC client connection
	imageSeviceClient = cri.NewImageServiceClient(conn)
	return
}

func CreatePod(pod apiobjects.Pod) (PodSandboxID string, err error) {

	// Parameters
	ctx := getContext()

	runtimeServiceClient, err := getRuntimeServiceClient()
	if err != nil {
		utils.Error("getRuntimeServiceClient error:", err)
		return
	}
	imageSeviceClient, err := getImageServiceClient()
	if err != nil {
		utils.Error("getImageServiceClient error:", err)
		return
	}

	// Create a pod sandbox
	sandboxConfig := cri.PodSandboxConfig{
		Metadata: &cri.PodSandboxMetadata{
			Name:      pod.ObjectMeta.Name,
			Namespace: pod.ObjectMeta.Namespace,
			Uid:       pod.UID,
		},
		Hostname: "",
		Labels:   pod.ObjectMeta.Labels,
		Annotations: make(map[string]string),
		Linux: &cri.LinuxPodSandboxConfig{},
		Windows: nil,
		PortMappings: nil,
	}
	runRequest := &cri.RunPodSandboxRequest{
		Config: &sandboxConfig,
	}
	response, err := runtimeServiceClient.RunPodSandbox(ctx, runRequest)
	//PodSandboxID = response.PodSandboxId
	if err != nil {
		utils.Error("RunPodSandbox error:", err)
		return
	}
	PodSandboxID = response.PodSandboxId
	utils.Info("Pod sandbox created with ID:", PodSandboxID)

	// Create containers
	for _, container := range pod.Spec.Containers {

		// Pull Image
		imageSpec := &cri.ImageSpec{
			Image: container.Image,
		}
		pullImageRequest := &cri.PullImageRequest{
			Image: imageSpec,
		}
		_, err = imageSeviceClient.PullImage(ctx, pullImageRequest)
		if err != nil {
			utils.Error("PullImage error:", err)
			return
		}
		utils.Info("Image pulled:", container.Image)

		// Create Container
		containerConfig := cri.ContainerConfig{
			Metadata: &cri.ContainerMetadata{
				Name: container.Name,
			},
			Image: &cri.ImageSpec{
				Image: container.Image,
			},
			Command: []string{"/bin/sh", "-c", "sleep 1000"},
			Args:    nil,
			WorkingDir: "/root",
			Envs: nil,
			Labels: container.Labels,
			Mounts: nil,
			Devices: nil,
		}

		sandboxConfig.Metadata.Attempt = 1;

		createContainerRequest := &cri.CreateContainerRequest{
			PodSandboxId:  PodSandboxID,
			Config:        &containerConfig,
			SandboxConfig: &sandboxConfig,
		}
		containerID, err1 := runtimeServiceClient.CreateContainer(ctx, createContainerRequest)

		err = err1
		if err != nil {
			utils.Error("CreateContainer error:", err)
			return
		}
		utils.Info("Container created with ID:", containerID)

		_, err = runtimeServiceClient.StartContainer(ctx, &cri.StartContainerRequest{ContainerId: containerID.ContainerId})
		if err != nil {
			utils.Error("StartContainer error:", err)
			return
		}
		utils.Info("Container started with ID:", containerID.ContainerId)
	}

	return
}

func convertCriContainerToMiniK8sContainer (response *cri.Container) (container apiobjects.Container) {
	container.Name = response.Metadata.Name
	container.Image = response.Image.Image
	container.Labels = response.Labels
	container.Status.State = apiobjects.ContainerState(response.State)
	container.Status.CreatedAt = response.CreatedAt
	return
}

func convertSandboxInfoToPod (response *cri.PodSandboxStatusResponse) (pod apiobjects.Pod) {
	pod.TypeMeta.ApiVersion = global.ApiVersion
	pod.TypeMeta.Kind = "Pod"
	pod.ObjectMeta.Name = response.Status.Metadata.Name
	pod.ObjectMeta.Namespace = response.Status.Metadata.Namespace
	pod.ObjectMeta.Labels = response.Status.Labels
	pod.ObjectMeta.UID = response.Status.Metadata.Uid

	pod.CreationTimestamp = utils.NanoUnixToTime(response.Status.CreatedAt)
	pod.Status.PodIP = response.Status.Network.Ip
	pod.Status.PodPhase = SandboxStateToPodPhase(response.Status.State)
	return
}

func GetPodInfo(sandboxId string) (response apiobjects.Pod, err error) {
	
	// Parameters
	ctx := getContext()

	runtimeServiceClient, err := getRuntimeServiceClient()
	if err != nil {
		utils.Error("getRuntimeServiceClient error:", err)
		return
	}

	// Get pod sandbox status
	podStatusRequest := &cri.PodSandboxStatusRequest{PodSandboxId: sandboxId,Verbose: true}
	var response_raw *cri.PodSandboxStatusResponse = nil
	response_raw, err = runtimeServiceClient.PodSandboxStatus(ctx, podStatusRequest)
	response = convertSandboxInfoToPod(response_raw)
	if err != nil {
		utils.Error("ListPodSandbox error:", err)
		return
	}

	// Get container status
	listRequest := &cri.ListContainersRequest{Filter: &cri.ContainerFilter{PodSandboxId: sandboxId}}
	response_containers, err := runtimeServiceClient.ListContainers(ctx, listRequest)
	if err != nil {
		utils.Error("ListContainers error:", err)
		return
	}

	for _, container := range response_containers.Containers {
		response.Spec.Containers = append(response.Spec.Containers, convertCriContainerToMiniK8sContainer(container))
	}

	utils.Debug("Pod sandbox status:", response)
	return 
}

func ListPods() (sandboxs []*cri.PodSandbox, err error) {

	// Parameters
	ctx := getContext()

	runtimeServiceClient, err := getRuntimeServiceClient()
	if err != nil {
		utils.Error("getRuntimeServiceClient error:", err)
		return
	}

	// List pod sandboxes
	listRequest := &cri.ListPodSandboxRequest{}
	response, err := runtimeServiceClient.ListPodSandbox(ctx, listRequest)
	if err != nil {
		utils.Error("ListPodSandbox error:", err)
		return
	}
	utils.Debug("Pod sandboxes:", response.Items)
	sandboxs = response.Items

	return
}
