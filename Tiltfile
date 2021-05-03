# -*- mode: Python -*-

# Settings and defaults.

project_name = 'newrelic-infra-operator'
release_name = 'dev'

settings = {
  'kind_cluster_name': 'kind',
  'live_reload': True,
  'chart_path': '../helm-charts-newrelic/charts/%s/' % project_name,
}

settings.update(read_json('tilt_option.json', default={}))

default_registry(settings.get('default_registry'))

# Only use explicitly allowed kubeconfigs as a safety measure.
allow_k8s_contexts(settings.get("allowed_contexts", "kind-" + settings.get('kind_cluster_name')))


# Building Docker image.
load('ext://restart_process', 'docker_build_with_restart')

if settings.get('live_reload'):
  # Building daemon binary locally.
  local_resource('%s-binary' % project_name, 'make build', deps=[
    './main.go',
    './internal',
  ])

  # Use custom Dockerfile for Tilt builds, which only takes locally built daemon binary for live reloading.
  dockerfile = '''
    FROM alpine:3.13
    COPY %s /usr/local/bin/
    ENTRYPOINT ["%s"]
  ''' % (project_name, project_name)

  docker_build_with_restart(project_name, '.',
    dockerfile_contents=dockerfile,
    entrypoint=project_name,
    only=project_name,
    live_update=[
      # Copy the binary so it gets restarted.
      sync(project_name, '/usr/local/bin/%s' % project_name),
    ],
  )
else:
  docker_build(project_name, '.')

# Deploying Kubernetes resources.
k8s_yaml(helm(settings.get('chart_path'), name=release_name, values='values-dev.yaml'))

# Tracking the deployment.
k8s_resource(release_name+'-newrelic-infra-operators')
