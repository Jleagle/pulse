load('ext://uibutton', 'cmd_button', 'location')

PULSE_DIR = '/Users/jameseagle/code/Jleagle/pulse'

# Entity 1: Compile Go binary for Linux
local_resource(
    'pulse-compile',
    cmd='cd %s && mkdir -p bin && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ./bin/pulse .' % PULSE_DIR,
    deps=[PULSE_DIR],
    ignore=[
        '%s/bin' % PULSE_DIR,
        '%s/pulse' % PULSE_DIR,
        '%s/.git' % PULSE_DIR,
        '%s/.idea' % PULSE_DIR,
        '%s/README.md' % PULSE_DIR,
    ],
    labels=['pulse']
)

# Entity 2: Run Docker container with pre-compiled binary
docker_build(
    'pulse-tilt',
    context=PULSE_DIR,
    dockerfile='%s/Dockerfile.tilt' % PULSE_DIR,
    only=[
        'bin/pulse'
    ],
    live_update=[
        sync('%s/bin/pulse' % PULSE_DIR, '/app/pulse'),
        restart_container()
    ]
)

docker_compose('%s/docker-compose.tilt.yml' % PULSE_DIR)
dc_resource('pulse', resource_deps=['pulse-compile'], labels=['pulse'])

# Custom UI Buttons to enable/disable the project in Tilt
cmd_button(
    'enable-pulse',
    argv=['tilt', 'enable', 'pulse-compile', 'pulse'],
    location=location.NAV,
    text='Enable Pulse',
    icon_name='play_arrow'
)
cmd_button(
    'disable-pulse',
    argv=['tilt', 'disable', 'pulse-compile', 'pulse'],
    location=location.NAV,
    text='Disable Pulse',
    icon_name='stop'
)
