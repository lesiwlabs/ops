package goapp

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"labs.lesiw.io/ops/goapp" // Application name is goapp.Name.
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sub"
)

var ctr, k8s, rnr, spkez *cmdio.Runner
var k8scfg string

type Ops struct {
	goapp.Ops

	Postgres bool   // Whether this app uses a PostgreSQL database.
	Hostname string // This application's public hostname, if it has one.
	Memory   int    // Request memory, in MB.
	Port     int    // Listening port.
	Scalable bool   // Whether this application can scale.
}

func (op Ops) Deploy() error {
	goapp.Targets = []goapp.Target{{
		Goos: "linux", Goarch: "arm",
		Unames: "linux", Unamer: "aarch64",
	}}
	if err := depanic(func() { op.Build() }); err != nil {
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

func (Ops) Destroy() error {
	// TODO: Make user type app name to destroy.
	helm := sub.WithRunner(ctr,
		"run", "-ti", "--rm",
		"-v", k8scfg+":/root/.kube/config",
		"-v", "helmcache:/root/.helm/cache",
		"alpine/helm",
	)
	err := helm.Run("uninstall", goapp.Name)
	if err != nil {
		return fmt.Errorf("could not delete helm release: %w", err)
	}
	// TODO: Delete Postgres role, if applicable.
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

// Temporary function to convert panics to errors.
// Should be removed once previously panicking functions are updated.
func depanic(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	f()
	return
}

func (op Ops) createImage(app string) (string, error) {
	file, err := os.Create("Dockerfile")
	if err != nil {
		return "", fmt.Errorf("could not create Dockerfile: %w", err)
	}
	defer func() { os.Remove(file.Name()) }()
	_, err = file.WriteString(fmt.Sprintf(`FROM scratch
COPY %s /app
CMD [ "/app" ]
`, app))
	if err != nil {
		return "", fmt.Errorf("could not write to Dockerfile: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("could not close Dockerfile: %w", err)
	}
	img := fmt.Sprintf("ctr.lesiw.dev/%s:%d", goapp.Name, time.Now().Unix())
	if err := ctr.Run("build", "-t", img, "."); err != nil {
		return "", fmt.Errorf("could not build container: %w", err)
	}
	err = cmdio.Pipe(
		spkez.Command("get", "ctr.lesiw.dev/auth"),
		ctr.Command("login", "--password-stdin", "-u", "ll", "ctr.lesiw.dev"),
	)
	if err != nil {
		return "", fmt.Errorf("could not docker login: %w", err)
	}
	if err := ctr.Run("push", img); err != nil {
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
      imagePullSecrets:
        - name: regcred
      containers:
        - name: app
          image: %[2]s
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: %[4]d
%[5]s
          resources:
            requests:
              memory: %[3]dMi
            limits:
              memory: %[3]dMi
`

// Application chart for a scalable app.
// 1: App
// 2: Image
// 3: Memory
// 4: Port
// 5: Additional container config
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
      imagePullSecrets:
        - name: regcred
      containers:
        - name: app
          image: %[2]s
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: %[4]d
%[5]s
          resources:
            requests:
              memory: %[3]dMi
            limits:
              memory: %[3]dMi
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
          env:
            - name: PGHOST
              value: db
            - name: PGUSER
              value: postgres
            - name: PGDATABASE
              value: postgres
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
// TODO: Edit this to set owner to created role.
// 1: App
// 2: Owner (not yet implemented)
const databaseChart = `---
apiVersion: postgresql.cnpg.io/v1
kind: Database
metadata:
  name: %[1]s
spec:
  name: %[1]s
  owner: app
  cluster:
    name: default
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
	defer func() { os.RemoveAll(chart) }()
	chart, err = filepath.Abs(chart)
	if err != nil {
		return fmt.Errorf("could not get full path of chart dir: %w", err)
	}
	err = os.WriteFile(
		filepath.Join(chart, "Chart.yaml"),
		[]byte(fmt.Sprintf(chartYaml, goapp.Name)),
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
	if op.Scalable {
		w.Printf(scalableAppChart,
			goapp.Name,
			img,
			cmp.Or(op.Memory, 32),
			cmp.Or(op.Port, 8080),
			z(op.Postgres, fmt.Sprintf(appPGChart, goapp.Name)),
		)
	} else {
		w.Printf(singleAppChart,
			goapp.Name,
			img,
			cmp.Or(op.Memory, 32),
			cmp.Or(op.Port, 8080),
			z(op.Postgres, fmt.Sprintf(appPGChart, goapp.Name)),
		)
	}
	w.Printf(serviceChart, goapp.Name, cmp.Or(op.Port, 8080))
	if op.Postgres {
		if err := createPostgresRole(goapp.Name); err != nil {
			return fmt.Errorf("failed to create postgres role: %w", err)
		}
		// TODO: Manually create app role and pass it in below.
		w.Printf(databaseChart, goapp.Name)
	}
	if op.Hostname != "" {
		w.Printf(ingressChart, goapp.Name, op.Hostname)
	}
	if w.err != nil {
		return fmt.Errorf("could not write to template file: %w", w.err)
	}
	helm := sub.WithRunner(ctr,
		"run", "-ti", "--rm",
		"-v", k8scfg+":/root/.kube/config",
		"-v", "helmcache:/root/.helm/cache",
		"-v", chart+":/chart",
		"alpine/helm",
	)
	err = helm.Run("upgrade", goapp.Name, "/chart", "--install")
	if err != nil {
		contents, readErr := os.ReadFile(filepath.Join(tmpl, "chart.yaml"))
		if readErr != nil {
			contents = []byte(fmt.Sprintf("<error reading file: %s>", readErr))
		}
		return fmt.Errorf(
			"could not helm install: %w\n---\nchart.yml:\n%s", err, contents,
		)
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
	secret := name + "-db-secret"
	err := k8s.Run("get", "secrets", secret)
	if err != nil {
		err = cmdio.Pipe(
			strings.NewReader(fmt.Sprintf(
				secretCfg, secret, "secret", randStr(32),
			)),
			k8s.Command("apply", "-f", "-"),
		)
		if err != nil {
			return fmt.Errorf("could not generate secret: %w", err)
		}
	}
	// TODO: Create actual role and use name argument in function.
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

func z[T any](b bool, a T) T {
	var zero T
	if b {
		return a
	}
	return zero
}
