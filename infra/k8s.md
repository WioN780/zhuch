# Kubernetes scale-out — k3d quickstart

Runs the headless arena fleet (`backend/Dockerfile.arena`) on a local k3d
cluster so training can hit N pods behind one Service instead of one process
on the host. Chart lives in `infra/helm/zhuch/`. This does not replace
`infra/compose/docker-compose.yml` (MLflow/Prometheus/Grafana stack) — run
that separately if you want tracking/dashboards; point the trainer at it via
`MLFLOW_TRACKING_URI` / `trainer.mlflowTrackingUri`, or use the chart's
optional in-cluster MLflow (`--set mlflow.enabled=true`, ephemeral, see
`values.yaml` comments).

Requires: Docker Desktop running, `k3d`, `kubectl`, `helm` on PATH.

## 1. Create the cluster

```powershell
k3d cluster create zhuch --agents 1
kubectl cluster-info
```

## 2. Build and import images

k3d clusters don't see your local Docker image cache — build normally, then
import into the cluster's containerd:

```powershell
docker build -f backend/Dockerfile.arena -t zhuch-arena:dev backend
k3d image import zhuch-arena:dev -c zhuch
```

Re-run both lines after every code change you want to test (`k3d image
import` again after rebuilding — there's no auto-sync).

## 3. Install the chart

```powershell
helm install zhuch infra/helm/zhuch `
  --set image.arena.repository=zhuch-arena `
  --set image.arena.tag=dev `
  --set image.arena.pullPolicy=IfNotPresent `
  --set arena.replicas=4

kubectl get pods -l app.kubernetes.io/component=arena -w
```

`pullPolicy=IfNotPresent` matters here — otherwise kubelet tries to pull
`zhuch-arena:dev` from a registry and fails, since it only exists in the
cluster's local image store via `k3d image import`.

## 4. Port-forward and train from the host

```powershell
kubectl port-forward svc/zhuch-arena 8081:8081
```

(Service name is `<release>-arena` off the `zhuch.fullname` helper — with
release name `zhuch` and chart name `zhuch` that collapses to `zhuch-arena`;
run `helm template` or `kubectl get svc` if unsure.)

In a second terminal:

```powershell
cd training
.venv\Scripts\python scripts\train.py --algo ga --generations 20 --arena-urls http://127.0.0.1:8081
```

A single port-forward to the Service is enough — kube-proxy load-balances
each HTTP request across all arena pods behind it (see the comment in
`templates/arena-service.yaml` for why this beats addressing pods
individually).

## 5. Scale

```powershell
kubectl scale deployment zhuch-arena --replicas=8
```

Or `helm upgrade zhuch infra/helm/zhuch --reuse-values --set arena.replicas=8`
if you want the chart's recorded state to match.

## 6. Run the trainer as an in-cluster Job (optional)

Needs a trainer image (not built by this workstream — build one from
`training/`, e.g. `FROM python:3.12-slim` + `pip install -e .[tracking]`,
then `k3d image import` it same as the arena image):

```powershell
helm upgrade zhuch infra/helm/zhuch --reuse-values `
  --set trainer.enabled=true `
  --set trainer.image.repository=zhuch-trainer `
  --set trainer.image.tag=dev `
  --set trainer.image.pullPolicy=IfNotPresent `
  --set trainer.generations=20

kubectl logs -f job/zhuch-trainer
```

## 7. Scaling benchmark

Goal: measure episodes/sec (eps/sec) the arena fleet sustains under a fixed
training workload, at 1, 2, 4, and 8 arena replicas, to find where the
trainer stops being arena-bound.

Procedure, for each replica count N:

```powershell
kubectl scale deployment zhuch-arena --replicas=N
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/component=arena --timeout=60s

cd training
.venv\Scripts\python scripts\train.py --algo ga --generations 10 --pop 256 --arena-urls http://127.0.0.1:8081
```

Keep `--generations`/`--pop` fixed across runs so total episode count is
comparable. Read eps/sec from:

- Arena pod logs (`kubectl logs -l app.kubernetes.io/component=arena`) if
  the arena logs per-episode timing, or
- MLflow, if `MLFLOW_TRACKING_URI` is set — training throughput is one of
  the dashboards already provisioned in
  `infra/grafana/dashboards/training-throughput.json` against the
  `arena_episodes_total` / `arena_episode_duration_seconds` metrics
  (contracts §8), or
- Wall-clock: (population × generations) / run duration, if neither of the
  above is wired up.

### Results

| Arena replicas | eps/sec | Notes |
|---|---|---|
| 1 | | |
| 2 | | |
| 4 | | |
| 8 | | |

Fill in after running — not measured as part of this workstream (live
cluster tools weren't available in this environment; see chart README /
verification notes).

## Cleanup

```powershell
helm uninstall zhuch
k3d cluster delete zhuch
```
