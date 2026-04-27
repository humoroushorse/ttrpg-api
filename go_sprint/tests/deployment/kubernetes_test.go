package deployment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// K8sDeployment represents a Kubernetes deployment manifest
type K8sDeployment struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string            `yaml:"name"`
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas int `yaml:"replicas"`
		Template struct {
			Spec struct {
				Containers []struct {
					Name  string `yaml:"name"`
					Image string `yaml:"image"`
					Ports []struct {
						ContainerPort int    `yaml:"containerPort"`
						Name          string `yaml:"name"`
					} `yaml:"ports"`
					LivenessProbe   *ProbeConfig     `yaml:"livenessProbe,omitempty"`
					ReadinessProbe  *ProbeConfig     `yaml:"readinessProbe,omitempty"`
					Resources       *Resources       `yaml:"resources,omitempty"`
					SecurityContext *SecurityContext `yaml:"securityContext,omitempty"`
				} `yaml:"containers"`
				SecurityContext *PodSecurityContext `yaml:"securityContext,omitempty"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type ProbeConfig struct {
	HTTPGet *struct {
		Path string `yaml:"path"`
		Port int    `yaml:"port"`
	} `yaml:"httpGet,omitempty"`
	InitialDelaySeconds int `yaml:"initialDelaySeconds"`
	PeriodSeconds       int `yaml:"periodSeconds"`
	TimeoutSeconds      int `yaml:"timeoutSeconds"`
	FailureThreshold    int `yaml:"failureThreshold"`
}

type Resources struct {
	Requests map[string]string `yaml:"requests"`
	Limits   map[string]string `yaml:"limits"`
}

type SecurityContext struct {
	AllowPrivilegeEscalation *bool  `yaml:"allowPrivilegeEscalation,omitempty"`
	ReadOnlyRootFilesystem   *bool  `yaml:"readOnlyRootFilesystem,omitempty"`
	RunAsNonRoot             *bool  `yaml:"runAsNonRoot,omitempty"`
	RunAsUser                *int64 `yaml:"runAsUser,omitempty"`
	Capabilities             *struct {
		Drop []string `yaml:"drop"`
	} `yaml:"capabilities,omitempty"`
}

type PodSecurityContext struct {
	RunAsNonRoot *bool  `yaml:"runAsNonRoot,omitempty"`
	RunAsUser    *int64 `yaml:"runAsUser,omitempty"`
	FSGroup      *int64 `yaml:"fsGroup,omitempty"`
}

// TestKubernetesManifestsExist verifies all required K8s manifests exist
func TestKubernetesManifestsExist(t *testing.T) {
	k8sDir := "../../k8s"

	requiredFiles := []string{
		"namespace.yaml",
		"configmap.yaml",
		"secrets.yaml",
		"sprint-service-deployment.yaml",
		"postgres-deployment.yaml",
		"nats-deployment.yaml",
		"keycloak-deployment.yaml",
		"auth-service-deployment.yaml",
		"ingress.yaml",
		"network-policy.yaml",
		"pod-disruption-budget.yaml",
		"service-monitor.yaml",
	}

	for _, file := range requiredFiles {
		path := filepath.Join(k8sDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Required Kubernetes manifest not found: %s", file)
		} else {
			t.Logf("Found manifest: %s", file)
		}
	}
}

// TestSprintServiceDeployment validates the sprint service deployment manifest
func TestSprintServiceDeployment(t *testing.T) {
	manifestPath := "../../k8s/sprint-service-deployment.yaml"

	// Read and parse the manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read deployment manifest: %v", err)
	}

	// Split YAML documents (file contains multiple resources)
	docs := strings.Split(string(data), "---")

	var deployment K8sDeployment
	foundDeployment := false

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var temp K8sDeployment
		if err := yaml.Unmarshal([]byte(doc), &temp); err != nil {
			continue // Skip non-deployment resources
		}

		if temp.Kind == "Deployment" {
			deployment = temp
			foundDeployment = true
			break
		}
	}

	if !foundDeployment {
		t.Fatal("Deployment resource not found in manifest")
	}

	// Test 1: Verify deployment metadata
	t.Run("Metadata", func(t *testing.T) {
		if deployment.Metadata.Name != "sprint-service" {
			t.Errorf("Expected deployment name 'sprint-service', got '%s'", deployment.Metadata.Name)
		}

		if deployment.Metadata.Namespace != "sprint-management" {
			t.Errorf("Expected namespace 'sprint-management', got '%s'", deployment.Metadata.Namespace)
		}

		if deployment.Metadata.Labels["app"] != "sprint-service" {
			t.Error("Deployment should have label 'app: sprint-service'")
		}
	})

	// Test 2: Verify replicas
	t.Run("Replicas", func(t *testing.T) {
		if deployment.Spec.Replicas < 1 {
			t.Error("Deployment should have at least 1 replica")
		}
		t.Logf("Deployment configured with %d replicas", deployment.Spec.Replicas)
	})

	// Test 3: Verify container configuration
	t.Run("Container", func(t *testing.T) {
		if len(deployment.Spec.Template.Spec.Containers) == 0 {
			t.Fatal("Deployment should have at least one container")
		}

		container := deployment.Spec.Template.Spec.Containers[0]

		if container.Name != "sprint-service" {
			t.Errorf("Expected container name 'sprint-service', got '%s'", container.Name)
		}

		if container.Image == "" {
			t.Error("Container image should be specified")
		}

		// Verify port
		foundPort := false
		for _, port := range container.Ports {
			if port.ContainerPort == 8080 {
				foundPort = true
				break
			}
		}
		if !foundPort {
			t.Error("Container should expose port 8080")
		}
	})

	// Test 4: Verify health probes
	t.Run("HealthProbes", func(t *testing.T) {
		container := deployment.Spec.Template.Spec.Containers[0]

		if container.LivenessProbe == nil {
			t.Error("Container should have liveness probe")
		} else {
			if container.LivenessProbe.HTTPGet == nil {
				t.Error("Liveness probe should use HTTP GET")
			} else if container.LivenessProbe.HTTPGet.Path != "/health/live" {
				t.Errorf("Liveness probe should check /health/live, got '%s'", container.LivenessProbe.HTTPGet.Path)
			}
		}

		if container.ReadinessProbe == nil {
			t.Error("Container should have readiness probe")
		} else {
			if container.ReadinessProbe.HTTPGet == nil {
				t.Error("Readiness probe should use HTTP GET")
			} else if container.ReadinessProbe.HTTPGet.Path != "/health/ready" {
				t.Errorf("Readiness probe should check /health/ready, got '%s'", container.ReadinessProbe.HTTPGet.Path)
			}
		}
	})

	// Test 5: Verify resource limits
	t.Run("Resources", func(t *testing.T) {
		container := deployment.Spec.Template.Spec.Containers[0]

		if container.Resources == nil {
			t.Error("Container should have resource requests and limits")
			return
		}

		if container.Resources.Requests == nil {
			t.Error("Container should have resource requests")
		} else {
			if _, ok := container.Resources.Requests["memory"]; !ok {
				t.Error("Container should have memory request")
			}
			if _, ok := container.Resources.Requests["cpu"]; !ok {
				t.Error("Container should have CPU request")
			}
		}

		if container.Resources.Limits == nil {
			t.Error("Container should have resource limits")
		} else {
			if _, ok := container.Resources.Limits["memory"]; !ok {
				t.Error("Container should have memory limit")
			}
			if _, ok := container.Resources.Limits["cpu"]; !ok {
				t.Error("Container should have CPU limit")
			}
		}
	})

	// Test 6: Verify security context
	t.Run("SecurityContext", func(t *testing.T) {
		container := deployment.Spec.Template.Spec.Containers[0]

		if container.SecurityContext == nil {
			t.Error("Container should have security context")
			return
		}

		if container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
			t.Error("Container should run as non-root")
		}

		if container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation {
			t.Error("Container should not allow privilege escalation")
		}

		if container.SecurityContext.RunAsUser == nil || *container.SecurityContext.RunAsUser == 0 {
			t.Error("Container should specify non-root user ID")
		}

		// Verify pod security context
		if deployment.Spec.Template.Spec.SecurityContext == nil {
			t.Error("Pod should have security context")
		} else {
			if deployment.Spec.Template.Spec.SecurityContext.RunAsNonRoot == nil || !*deployment.Spec.Template.Spec.SecurityContext.RunAsNonRoot {
				t.Error("Pod should run as non-root")
			}
		}
	})
}

// TestKubernetesManifestValidation validates all K8s manifests using kubectl
func TestKubernetesManifestValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Kubernetes validation test in short mode")
	}

	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl not found in PATH, skipping Kubernetes validation")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	k8sDir := "../../k8s"

	// Get all YAML files
	files, err := filepath.Glob(filepath.Join(k8sDir, "*.yaml"))
	if err != nil {
		t.Fatalf("Failed to list K8s manifests: %v", err)
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		t.Run(fmt.Sprintf("Validate_%s", fileName), func(t *testing.T) {
			// Use kubectl dry-run to validate manifest
			cmd := exec.CommandContext(ctx, "kubectl", "apply", "--dry-run=client", "-f", file)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("Manifest validation failed for %s: %v\nOutput: %s", fileName, err, string(output))
			} else {
				t.Logf("Manifest %s is valid", fileName)
			}
		})
	}
}

// TestKubernetesDeploymentScripts tests the deployment scripts
func TestKubernetesDeploymentScripts(t *testing.T) {
	scriptsToTest := []struct {
		name string
		path string
	}{
		{"deploy.sh", "../../k8s/deploy.sh"},
		{"validate.sh", "../../k8s/validate.sh"},
	}

	for _, script := range scriptsToTest {
		t.Run(script.name, func(t *testing.T) {
			// Check script exists
			if _, err := os.Stat(script.path); os.IsNotExist(err) {
				t.Errorf("Script not found: %s", script.path)
				return
			}

			// Check script is executable
			info, err := os.Stat(script.path)
			if err != nil {
				t.Fatalf("Failed to stat script: %v", err)
			}

			mode := info.Mode()
			if mode&0111 == 0 {
				t.Errorf("Script %s is not executable", script.name)
			} else {
				t.Logf("Script %s is executable", script.name)
			}

			// Read script content
			content, err := os.ReadFile(script.path)
			if err != nil {
				t.Fatalf("Failed to read script: %v", err)
			}

			scriptContent := string(content)

			// Verify shebang
			if !strings.HasPrefix(scriptContent, "#!/") {
				t.Errorf("Script %s should have shebang", script.name)
			}

			// Verify it references kubectl
			if !strings.Contains(scriptContent, "kubectl") {
				t.Errorf("Script %s should use kubectl", script.name)
			}
		})
	}
}

// TestNamespaceConfiguration validates the namespace manifest
func TestNamespaceConfiguration(t *testing.T) {
	manifestPath := "../../k8s/namespace.yaml"

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read namespace manifest: %v", err)
	}

	var namespace struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name   string            `yaml:"name"`
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
	}

	if err := yaml.Unmarshal(data, &namespace); err != nil {
		t.Fatalf("Failed to parse namespace manifest: %v", err)
	}

	if namespace.Kind != "Namespace" {
		t.Errorf("Expected kind 'Namespace', got '%s'", namespace.Kind)
	}

	if namespace.Metadata.Name != "sprint-management" {
		t.Errorf("Expected namespace 'sprint-management', got '%s'", namespace.Metadata.Name)
	}

	t.Logf("Namespace configuration is valid")
}

// TestServiceConfiguration validates the service manifest
func TestServiceConfiguration(t *testing.T) {
	manifestPath := "../../k8s/sprint-service-deployment.yaml"

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read service manifest: %v", err)
	}

	// Split YAML documents
	docs := strings.Split(string(data), "---")

	var service struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name      string            `yaml:"name"`
			Namespace string            `yaml:"namespace"`
			Labels    map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
		Spec struct {
			Type     string            `yaml:"type"`
			Selector map[string]string `yaml:"selector"`
			Ports    []struct {
				Port       int    `yaml:"port"`
				TargetPort int    `yaml:"targetPort"`
				Protocol   string `yaml:"protocol"`
				Name       string `yaml:"name"`
			} `yaml:"ports"`
		} `yaml:"spec"`
	}

	foundService := false
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var temp struct {
			Kind string `yaml:"kind"`
		}
		if err := yaml.Unmarshal([]byte(doc), &temp); err != nil {
			continue
		}

		if temp.Kind == "Service" {
			if err := yaml.Unmarshal([]byte(doc), &service); err != nil {
				t.Fatalf("Failed to parse service manifest: %v", err)
			}
			foundService = true
			break
		}
	}

	if !foundService {
		t.Fatal("Service resource not found in manifest")
	}

	if service.Metadata.Name != "sprint-service" {
		t.Errorf("Expected service name 'sprint-service', got '%s'", service.Metadata.Name)
	}

	if service.Spec.Type != "ClusterIP" {
		t.Errorf("Expected service type 'ClusterIP', got '%s'", service.Spec.Type)
	}

	if len(service.Spec.Ports) == 0 {
		t.Error("Service should have at least one port")
	}

	t.Logf("Service configuration is valid")
}

// TestHPAConfiguration validates the HorizontalPodAutoscaler manifest
func TestHPAConfiguration(t *testing.T) {
	manifestPath := "../../k8s/sprint-service-deployment.yaml"

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read HPA manifest: %v", err)
	}

	// Split YAML documents
	docs := strings.Split(string(data), "---")

	var hpa struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			ScaleTargetRef struct {
				APIVersion string `yaml:"apiVersion"`
				Kind       string `yaml:"kind"`
				Name       string `yaml:"name"`
			} `yaml:"scaleTargetRef"`
			MinReplicas int `yaml:"minReplicas"`
			MaxReplicas int `yaml:"maxReplicas"`
		} `yaml:"spec"`
	}

	foundHPA := false
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var temp struct {
			Kind string `yaml:"kind"`
		}
		if err := yaml.Unmarshal([]byte(doc), &temp); err != nil {
			continue
		}

		if temp.Kind == "HorizontalPodAutoscaler" {
			if err := yaml.Unmarshal([]byte(doc), &hpa); err != nil {
				t.Fatalf("Failed to parse HPA manifest: %v", err)
			}
			foundHPA = true
			break
		}
	}

	if !foundHPA {
		t.Fatal("HorizontalPodAutoscaler resource not found in manifest")
	}

	if hpa.Spec.ScaleTargetRef.Name != "sprint-service" {
		t.Errorf("HPA should target 'sprint-service', got '%s'", hpa.Spec.ScaleTargetRef.Name)
	}

	if hpa.Spec.MinReplicas < 1 {
		t.Error("HPA should have at least 1 minimum replica")
	}

	if hpa.Spec.MaxReplicas <= hpa.Spec.MinReplicas {
		t.Error("HPA max replicas should be greater than min replicas")
	}

	t.Logf("HPA configuration is valid (min: %d, max: %d)", hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas)
}

// TestConfigMapExists validates the ConfigMap manifest
func TestConfigMapExists(t *testing.T) {
	manifestPath := "../../k8s/configmap.yaml"

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read ConfigMap manifest: %v", err)
	}

	var configMap struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Data map[string]string `yaml:"data"`
	}

	if err := yaml.Unmarshal(data, &configMap); err != nil {
		t.Fatalf("Failed to parse ConfigMap manifest: %v", err)
	}

	if configMap.Kind != "ConfigMap" {
		t.Errorf("Expected kind 'ConfigMap', got '%s'", configMap.Kind)
	}

	if len(configMap.Data) == 0 {
		t.Error("ConfigMap should have data entries")
	}

	t.Logf("ConfigMap configuration is valid with %d entries", len(configMap.Data))
}
