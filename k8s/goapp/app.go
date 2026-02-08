package goapp

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"labs.lesiw.io/ops/goapp" // Application name is goapp.Name.
	"lesiw.io/command"
	"lesiw.io/command/sub"
)

var spkez, ctr, k8s command.Machine
var k8scfg string

type Ops struct {
	goapp.Ops

	Postgres       bool   // Whether this app uses a PostgreSQL database.
	Hostname       string // This application's public hostname, if it has one.
	Memory         int    // Requested memory, in MB.
	Port           int    // Listening port.
	Scalable       bool   // Whether this application can scale.
	ServiceAccount string // K8s service account.
	K8sDefinitions string // Additional k8s definitions.

	Env        map[string]string // Map of environment variables.
	EnvSecrets map[string]string // Map of spkez secrets, exposed as env vars.
}

func (op Ops) Deploy() error {
	goapp.Targets = []goapp.Target{{
		Goos: "linux", Goarch: "arm",
		Unames: "linux", Unamer: "aarch64",
	}}
	if err := op.Build(); err != nil {
		return fmt.Errorf("could not build app: %w", err)
	}
	img, err := op.createImage(filepath.Join(
		"out", goapp.Name+"-linux-aarch64"))
	if err != nil {
		return fmt.Errorf("could not create container: %w", err)
	}
	if err := op.deployImage(img); err != nil {
		return fmt.Errorf("could not create helm chart: %w", err)
	}
	return nil
}

func (op Ops) Destroy() error {
	return op.destroy(false)
}

func (op Ops) ForceDestroy() error {
	return op.destroy(true)
}

func (op Ops) destroy(force bool) error {
	ctx := context.Background()
	// TODO: Make user type app name to destroy.
	if !force {
		if err := op.Backup(); err != nil {
			return fmt.Errorf("could not backup the application: %w", err)
		}
	}
	helm := sub.Machine(ctr, "run", "-ti", "--rm",
		"-v", k8scfg+":/root/.kube/config",
		"-v", "helmcache:/root/.helm/cache",
		"alpine/helm",
	)
	err := command.Exec(ctx, helm, "uninstall", goapp.Name)
	if err != nil {
		return fmt.Errorf("could not delete helm release: %w", err)
	}
	if op.Postgres {
		pg := sub.Machine(k8s,
			"exec", "postgres-1", "-c", "postgres", "--", "psql", "-c",
		)
		err := command.Exec(ctx, pg, fmt.Sprintf("drop role %s;", goapp.Name))
		if err != nil {
			return fmt.Errorf("could not drop postgres role: %w", err)
		}
	}
	return nil
}

func (Ops) Backup() error {
	// TODO: Back up the application data.
	return nil
}

func (Ops) Restore() error {
	// TODO: Restore the application data from backup.
	return nil
}

func (op Ops) createImage(app string) (string, error) {
	ctx := context.Background()
	file, err := os.Create("Dockerfile")
	if err != nil {
		return "", fmt.Errorf("could not create Dockerfile: %w", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()
	_, err = fmt.Fprintf(file, `FROM scratch
COPY %s /app
CMD [ "/app" ]
`, app)
	if err != nil {
		return "", fmt.Errorf("could not write to Dockerfile: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("could not close Dockerfile: %w", err)
	}
	img := fmt.Sprintf("ctr.lesiw.dev/%s:%d", goapp.Name, time.Now().Unix())
	if err := command.Exec(ctx, ctr, "build", "-t", img, "."); err != nil {
		return "", fmt.Errorf("could not build container: %w", err)
	}
	_, err = io.Copy(
		command.NewWriter(ctx, ctr, "login",
			"--password-stdin", "-u", "ll", "ctr.lesiw.dev"),
		command.NewReader(ctx, spkez, "get", "ctr.lesiw.dev/auth"),
	)
	if err != nil {
		return "", fmt.Errorf("could not docker login: %w", err)
	}
	if err := command.Exec(ctx, ctr, "push", img); err != nil {
		return "", fmt.Errorf("could not push container: %w", err)
	}
	return img, nil
}

// Chart.yaml template.
// 1: App
const chartYaml = `apiVersion: v2
name: %[1]s
type: application
version: 1.0.0
`

// Application chart for a singleton app.
// 1: App
// 2: Image
// 3: Memory
// 4: Port
// 5: Additional container config
// 6: Service account
const singleAppChart = `---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: %[1]s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %[1]s
  template:
    metadata:
      labels:
        app: %[1]s
    spec:
      serviceAccountName: %[4]s
      imagePullSecrets:
        - name: regcred
      containers:
        - name: app
          image: %[2]s
          imagePullPolicy: IfNotPresent
          resources:
            requests:
              memory: %[3]dMi
            limits:
              memory: %[3]dMi
%[5]s
`

// Application chart for a scalable app.
// 1: App
// 2: Image
// 3: Memory
// 4: Port
// 5: Additional container config
// 6: Service account
const scalableAppChart = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
spec:
  replicas: 2
  selector:
    matchLabels:
      app: %[1]s
  template:
    metadata:
      labels:
        app: %[1]s
    spec:
	  serviceAccountName: %[4]s
      imagePullSecrets:
        - name: regcred
      containers:
        - name: app
          image: %[2]s
          imagePullPolicy: IfNotPresent
          resources:
            requests:
              memory: %[3]dMi
            limits:
              memory: %[3]dMi
%[5]s
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: %[1]s
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: %[1]s
  minReplicas: 2
  maxReplicas: 5
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 80
`

// Application chart partial: Postgres config.
// 1: App
const appPGChart = `
            - name: PGHOST
              value: postgres-rw
            - name: PGUSER
              value: %[1]s
            - name: PGDATABASE
              value: %[1]s
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: %[1]s-db-secret
                  key: secret
`

// Service chart.
// 1: App
// 2: Port
const serviceChart = `---
apiVersion: v1
kind: Service
metadata:
  name: %[1]s
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: %[2]d
  selector:
    app: %[1]s
`

// Database chart.
// 1: App
// 2: Owner
const databaseChart = `---
apiVersion: postgresql.cnpg.io/v1
kind: Database
metadata:
  name: %[1]s
spec:
  databaseReclaimPolicy: delete
  name: %[1]s
  owner: %[2]s
  cluster:
    name: postgres
`

// Ingress chart.
// 1: App
// 2: Hostname
const ingressChart = `---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: %[2]s
spec:
  secretName: %[2]s
  dnsNames:
    - %[2]s
  issuerRef:
    name: cloudflare-issuer
    kind: Issuer
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %[1]s-ingress
  annotations:
    traefik.ingress.kubernetes.io/router.tls: "true"
    traefik.ingress.kubernetes.io/frontend.entryPoints.websecure: websecure
spec:
  tls:
    - hosts:
      - %[2]s
      secretName: %[2]s
  rules:
    - host: %[2]s
      http:
        paths:
          - backend:
              service:
                name: %[1]s
                port:
                  number: 80
            path: /
            pathType: Prefix
`

func (op Ops) deployImage(img string) error {
	chart, err := os.MkdirTemp("", "chart")
	if err != nil {
		return fmt.Errorf("could not create temporary directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(chart) }()
	chart, err = filepath.Abs(chart)
	if err != nil {
		return fmt.Errorf("could not get full path of chart dir: %w", err)
	}
	err = os.WriteFile(
		filepath.Join(chart, "Chart.yaml"),
		fmt.Appendf(nil, chartYaml, goapp.Name),
		0755,
	)
	if err != nil {
		return fmt.Errorf("could not create Chart.yaml: %w", err)
	}
	tmpl := filepath.Join(chart, "templates")
	if err := os.Mkdir(tmpl, 0755); err != nil {
		return fmt.Errorf("could not create templates directory: %w", err)
	}
	cfg, err := os.Create(filepath.Join(tmpl, "chart.yaml"))
	if err != nil {
		return fmt.Errorf("could not create chart template file: %w", err)
	}
	w := errWriter{w: cfg}
	ctrspec, err := op.k8sCtrSpec()
	if err != nil {
		return fmt.Errorf("could not create environment block: %w", err)
	}
	args := []any{
		goapp.Name, img, cmp.Or(op.Memory, 32),
		cmp.Or(op.ServiceAccount, "default"), ctrspec,
	}
	if op.Scalable {
		w.Printf(scalableAppChart, args...)
	} else {
		w.Printf(singleAppChart, args...)
	}
	if op.Port > 0 {
		w.Printf(serviceChart, goapp.Name, op.Port)
	}
	if op.Postgres {
		if err := createPostgresRole(goapp.Name); err != nil {
			return fmt.Errorf("failed to create postgres role: %w", err)
		}
		w.Printf(databaseChart, goapp.Name, goapp.Name)
	}
	if op.Hostname != "" {
		w.Printf(ingressChart, goapp.Name, op.Hostname)
	}
	w.Printf("%s", op.K8sDefinitions)
	if w.err != nil {
		return fmt.Errorf("could not write to template file: %w", w.err)
	}
	ctx := context.Background()
	helm := sub.Machine(ctr, "run", "-ti", "--rm",
		"-v", k8scfg+":/root/.kube/config",
		"-v", "helmcache:/root/.helm/cache",
		"-v", chart+":/chart",
		"alpine/helm",
	)
	err = command.Exec(ctx, helm, "upgrade", goapp.Name, "/chart", "--install")
	if err != nil {
		contents, readErr := os.ReadFile(filepath.Join(tmpl, "chart.yaml"))
		if readErr != nil {
			contents = fmt.Appendf(nil,
				"<error reading file: %s>", readErr)
		}
		return fmt.Errorf(
			"could not helm install: %w\n---\nchart.yml:\n%s", err, contents,
		)
	}
	return nil
}

func (op Ops) k8sCtrSpec() (string, error) {
	ctx := context.Background()
	var spec strings.Builder

	var env strings.Builder
	if op.Postgres {
		env.WriteString(fmt.Sprintf(appPGChart, goapp.Name))
	}
	for k, v := range op.Env {
		env.WriteString(fmt.Sprintf("            - name: %s\n"+
			"            value: %s\n", k, v))
	}
	for k, v := range op.EnvSecrets {
		r, err := command.Read(ctx, spkez, "get", v)
		if err != nil {
			return "", fmt.Errorf("could not read secret %q: %w", v, err)
		}
		name := regexp.MustCompile(`[^a-zA-Z0-9]+`).
			ReplaceAllString(v, ".")
		if err := op.writeSecret(name, r); err != nil {
			return "", fmt.Errorf("could not store secret %q: %w", v, err)
		}
		env.WriteString(fmt.Sprintf("            - name: %s\n"+
			"              valueFrom:\n"+
			"                secretKeyRef:\n"+
			"                  name: %s\n"+
			"                  key: data\n", k, name))
	}
	if env.Len() > 0 {
		spec.WriteString("          env:\n" + env.String())
	}

	return spec.String(), nil
}

func (op Ops) writeSecret(k, v string) error {
	ctx := context.Background()
	_, err := io.Copy(
		command.NewWriter(ctx, k8s, "apply", "-f", "-"),
		strings.NewReader(fmt.Sprintf(secretCfg, k, "data", v)),
	)
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}
	return nil
}

const secretCfg = `apiVersion: v1
kind: Secret
metadata:
  name: %s
type: Opaque
stringData:
  %s: %s`

func createPostgresRole(name string) error {
	ctx := context.Background()
	secretName := name + "-db-secret"
	var secretPass string
	err := command.Do(ctx, k8s, "get", "secrets", secretName)
	if err != nil {
		secretPass = randStr(32)
		_, err = io.Copy(
			command.NewWriter(ctx, k8s, "apply", "-f", "-"),
			strings.NewReader(fmt.Sprintf(
				secretCfg, secretName, "secret", secretPass)),
		)
		if err != nil {
			return fmt.Errorf("could not generate secret: %w", err)
		}
	} else {
		encoded, err := command.Read(ctx, k8s,
			"get", "secret", secretName,
			"-o", "jsonpath={.data.secret}",
		)
		if err != nil {
			return fmt.Errorf("could not get postgres password: %w", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("could not decode postgres password: %w", err)
		}
		secretPass = string(decoded)
	}
	pg := sub.Machine(k8s,
		"exec", "postgres-1", "-c", "postgres", "--", "psql", "-c",
	)
	sql := `DO
$do$
BEGIN
   IF EXISTS (
      SELECT FROM pg_catalog.pg_roles
      WHERE rolname = '%[1]s') THEN

      RAISE NOTICE 'Role "%[1]s" already exists. Skipping.';
   ELSE
      CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s';
   END IF;
END
$do$;
`
	// I'd welcome some sanitization here.
	// Most PostgreSQL libraries don't expose their sanitization methods,
	// and it would be nice to keep any import used here lightweight
	// since I only need to run this single query.
	// goapp.Name is trusted in any case and could already be used
	// for k8s config injection and other terrible things,
	// so this isn't an immediate concern.
	err = command.Exec(ctx, pg, fmt.Sprintf(sql, name, secretPass))
	if err != nil {
		return fmt.Errorf("could not create role: %w", err)
	}
	return nil
}

const (
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func randStr(n int) string {
	var b strings.Builder
	for range n {
		b.WriteByte(alphabet[rand.IntN(len(alphabet))])
	}
	return b.String()
}

type errWriter struct {
	w   interface{ WriteString(string) (int, error) }
	err error
}

func (w *errWriter) Printf(format string, a ...any) {
	if w.err != nil {
		return
	}
	_, w.err = w.w.WriteString(fmt.Sprintf(format, a...))
}
