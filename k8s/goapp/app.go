package goapp

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
	"time"

	"labs.lesiw.io/ops/goapp"
	"lesiw.io/command"
	"lesiw.io/command/ctr"
	"lesiw.io/command/sub"
	"lesiw.io/command/sys"
	"lesiw.io/defers"
)

var ctl = ctr.Ctl(sys.Machine())

var getSpkez = sync.OnceValues(func() (command.Machine, error) {
	ctx := context.Background()
	sh := command.Shell(sys.Machine())
	sh.Handle("go", sh.Unshell())
	sh.Handle("spkez", sh.Unshell())

	err := sh.Do(ctx, "spkez", "--version")
	if command.NotFound(err) {
		err := sh.Exec(ctx, "go", "install", "lesiw.io/spkez@latest")
		if err != nil {
			return nil, fmt.Errorf("could not install spkez: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("error checking spkez: %w", err)
	}

	return sub.Machine(sh, "spkez"), nil
})

var getKubectl = sync.OnceValues(func() (command.Machine, error) {
	ctx := context.Background()
	spkez, err := getSpkez()
	if err != nil {
		return nil, err
	}
	m := ctr.Machine(sys.Machine(), "bitnami/kubectl", "--entrypoint", "")
	defers.Add(func() { _ = command.Shutdown(context.Background(), m) })
	_, err = command.Copy(
		command.NewWriter(ctx, m,
			"sh", "-c", "mkdir -p /.kube && cat > /.kube/config"),
		command.NewReader(ctx, spkez, "get", "k8s/config"),
	)
	if err != nil {
		return nil, fmt.Errorf("could not set kubeconfig: %w", err)
	}
	return sub.Machine(m, "kubectl"), nil
})

var getHelm = sync.OnceValues(func() (command.Machine, error) {
	ctx := context.Background()
	spkez, err := getSpkez()
	if err != nil {
		return nil, err
	}
	m := ctr.Machine(sys.Machine(), "alpine/helm", "--entrypoint", "")
	defers.Add(func() { _ = command.Shutdown(context.Background(), m) })
	_, err = command.Copy(
		command.NewWriter(ctx, m,
			"sh", "-c", "mkdir -p /root/.kube && cat > /root/.kube/config"),
		command.NewReader(ctx, spkez, "get", "k8s/config"),
	)
	if err != nil {
		return nil, fmt.Errorf("could not set kubeconfig: %w", err)
	}
	return m, nil
})

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

func (op Ops) Deploy(ctx context.Context) error {
	goapp.Targets = []goapp.Target{{
		Goos: "linux", Goarch: "arm",
		Unames: "linux", Unamer: "aarch64",
	}}
	if err := op.Build(ctx); err != nil {
		return fmt.Errorf("could not build app: %w", err)
	}
	sh := command.Shell(sys.Machine())
	err := sh.Rename(ctx,
		"out/"+goapp.Name+"-linux-aarch64", "out/app")
	if err != nil {
		return fmt.Errorf("could not rename binary: %w", err)
	}
	img, err := op.createImage(ctx, sh)
	if err != nil {
		return fmt.Errorf("could not create container: %w", err)
	}
	if err := op.deployImage(ctx, img); err != nil {
		return fmt.Errorf("could not create helm chart: %w", err)
	}
	return nil
}

func (op Ops) Destroy(ctx context.Context) error {
	return op.destroy(ctx, false)
}

func (op Ops) ForceDestroy(ctx context.Context) error {
	return op.destroy(ctx, true)
}

func (op Ops) destroy(ctx context.Context, force bool) error {
	if !force {
		if err := op.Backup(ctx); err != nil {
			return fmt.Errorf("could not backup the application: %w", err)
		}
	}
	helm, err := getHelm()
	if err != nil {
		return err
	}
	err = command.Exec(ctx, helm, "helm", "uninstall", goapp.Name)
	if err != nil {
		return fmt.Errorf("could not delete helm release: %w", err)
	}
	if op.Postgres {
		kubectl, err := getKubectl()
		if err != nil {
			return err
		}
		pg := sub.Machine(kubectl,
			"exec", "postgres-1", "-c", "postgres", "--", "psql", "-c",
		)
		err = command.Exec(ctx, pg,
			fmt.Sprintf("drop role %s;", goapp.Name))
		if err != nil {
			return fmt.Errorf("could not drop postgres role: %w", err)
		}
	}
	return nil
}

func (Ops) Backup(_ context.Context) error {
	// TODO: Back up the application data.
	return nil
}

func (Ops) Restore(_ context.Context) error {
	// TODO: Restore the application data from backup.
	return nil
}

func (op Ops) createImage(
	ctx context.Context, sh *command.Sh,
) (string, error) {
	img := fmt.Sprintf("ctr.lesiw.dev/%s:%d",
		goapp.Name, time.Now().Unix())
	_, err := command.Copy(
		command.NewWriter(ctx, ctl,
			"import", "--change", `CMD ["/app"]`, "-", img),
		sh.OpenBuffer(ctx, "out/"),
	)
	if err != nil {
		return "", fmt.Errorf("could not import container: %w", err)
	}
	spkez, err := getSpkez()
	if err != nil {
		return "", err
	}
	_, err = command.Copy(
		command.NewWriter(ctx, ctl, "login",
			"--password-stdin", "-u", "ll", "ctr.lesiw.dev"),
		command.NewReader(ctx, spkez, "get", "ctr.lesiw.dev/auth"),
	)
	if err != nil {
		return "", fmt.Errorf("could not docker login: %w", err)
	}
	if err := command.Exec(ctx, ctl, "push", img); err != nil {
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

func (op Ops) deployImage(ctx context.Context, img string) error {
	helm, err := getHelm()
	if err != nil {
		return err
	}
	err = command.Exec(ctx, helm, "mkdir", "-p", "/chart/templates")
	if err != nil {
		return fmt.Errorf("could not create chart directory: %w", err)
	}
	_, err = command.Copy(
		command.NewWriter(ctx, helm,
			"sh", "-c", "cat > /chart/Chart.yaml"),
		strings.NewReader(fmt.Sprintf(chartYaml, goapp.Name)),
	)
	if err != nil {
		return fmt.Errorf("could not create Chart.yaml: %w", err)
	}
	ctrspec, err := op.k8sCtrSpec(ctx)
	if err != nil {
		return fmt.Errorf("could not create environment block: %w", err)
	}
	var tmpl strings.Builder
	w := errWriter{w: &tmpl}
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
		if err := createPostgresRole(ctx, goapp.Name); err != nil {
			return fmt.Errorf("failed to create postgres role: %w", err)
		}
		w.Printf(databaseChart, goapp.Name, goapp.Name)
	}
	if op.Hostname != "" {
		w.Printf(ingressChart, goapp.Name, op.Hostname)
	}
	w.Printf("%s", op.K8sDefinitions)
	if w.err != nil {
		return fmt.Errorf("could not build template: %w", w.err)
	}
	_, err = command.Copy(
		command.NewWriter(ctx, helm,
			"sh", "-c", "cat > /chart/templates/chart.yaml"),
		strings.NewReader(tmpl.String()),
	)
	if err != nil {
		return fmt.Errorf("could not write chart template: %w", err)
	}
	err = command.Exec(ctx, helm,
		"helm", "upgrade", goapp.Name, "/chart", "--install")
	if err != nil {
		chart, readErr := command.Read(ctx, helm,
			"cat", "/chart/templates/chart.yaml")
		if readErr != nil {
			chart = fmt.Sprintf("<error reading file: %s>", readErr)
		}
		return fmt.Errorf(
			"could not helm install: %w\n---\nchart.yml:\n%s", err, chart)
	}
	_ = command.Do(ctx, helm, "rm", "-rf", "/chart")
	return nil
}

func (op Ops) k8sCtrSpec(ctx context.Context) (string, error) {
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
		spkez, err := getSpkez()
		if err != nil {
			return "", err
		}
		r, err := command.Read(ctx, spkez, "get", v)
		if err != nil {
			return "", fmt.Errorf("could not read secret %q: %w", v, err)
		}
		name := regexp.MustCompile(`[^a-zA-Z0-9]+`).
			ReplaceAllString(v, ".")
		if err := op.writeSecret(ctx, name, r); err != nil {
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

func (op Ops) writeSecret(ctx context.Context, k, v string) error {
	kubectl, err := getKubectl()
	if err != nil {
		return err
	}
	_, err = command.Copy(
		command.NewWriter(ctx, kubectl, "apply", "-f", "-"),
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

func createPostgresRole(ctx context.Context, name string) error {
	kubectl, err := getKubectl()
	if err != nil {
		return err
	}
	secretName := name + "-db-secret"
	var secretPass string
	err = command.Do(ctx, kubectl, "get", "secrets", secretName)
	if err != nil {
		secretPass = randStr(32)
		_, err = command.Copy(
			command.NewWriter(ctx, kubectl, "apply", "-f", "-"),
			strings.NewReader(fmt.Sprintf(
				secretCfg, secretName, "secret", secretPass)),
		)
		if err != nil {
			return fmt.Errorf("could not generate secret: %w", err)
		}
	} else {
		encoded, err := command.Read(ctx, kubectl,
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
	pg := sub.Machine(kubectl,
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
