from collections import namedtuple


CONFIG_KEY_NAMES = [
    'AGENT_HOSTNAME',
    'AGENT_HTTPS',
    'AGENT_PORT',
    'DEBUG',
    'ENABLED',
    'GLOBAL_TAGS',
    'SAMPLER',
    'PRIORITY_SAMPLING',
    'SETTINGS',
]

# Keys used for the configuration dict
ConfigKeyNames = namedtuple('ConfigKeyNames', CONFIG_KEY_NAMES)

ConfigKeys = ConfigKeyNames(
    AGENT_HOSTNAME='agent_hostname',
    AGENT_HTTPS='agent_https',
    AGENT_PORT='agent_port',
    DEBUG='debug',
    ENABLED='enabled',
    GLOBAL_TAGS='global_tags',
    SAMPLER='sampler',
    PRIORITY_SAMPLING='priority_sampling',
    SETTINGS='settings',
)


def config_invalid_keys(config):
    """Returns a list of keys that exist in *config* and not in KEYS."""
    return [key for key in config.keys() if key not in ConfigKeys]
