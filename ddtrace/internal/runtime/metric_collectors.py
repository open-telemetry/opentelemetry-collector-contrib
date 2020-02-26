import os

from .collector import ValueCollector
from .constants import (
    GC_COUNT_GEN0,
    GC_COUNT_GEN1,
    GC_COUNT_GEN2,
    THREAD_COUNT,
    MEM_RSS,
    CTX_SWITCH_VOLUNTARY,
    CTX_SWITCH_INVOLUNTARY,
    CPU_TIME_SYS,
    CPU_TIME_USER,
    CPU_PERCENT,
)


class RuntimeMetricCollector(ValueCollector):
    value = []
    periodic = True


class GCRuntimeMetricCollector(RuntimeMetricCollector):
    """ Collector for garbage collection generational counts

    More information at https://docs.python.org/3/library/gc.html
    """

    required_modules = ["gc"]

    def collect_fn(self, keys):
        gc = self.modules.get("gc")

        counts = gc.get_count()
        metrics = [
            (GC_COUNT_GEN0, counts[0]),
            (GC_COUNT_GEN1, counts[1]),
            (GC_COUNT_GEN2, counts[2]),
        ]

        return metrics


class PSUtilRuntimeMetricCollector(RuntimeMetricCollector):
    """Collector for psutil metrics.

    Performs batched operations via proc.oneshot() to optimize the calls.
    See https://psutil.readthedocs.io/en/latest/#psutil.Process.oneshot
    for more information.
    """

    required_modules = ["psutil"]
    stored_value = dict(
        CPU_TIME_SYS_TOTAL=0, CPU_TIME_USER_TOTAL=0, CTX_SWITCH_VOLUNTARY_TOTAL=0, CTX_SWITCH_INVOLUNTARY_TOTAL=0,
    )

    def _on_modules_load(self):
        self.proc = self.modules["psutil"].Process(os.getpid())

    def collect_fn(self, keys):
        with self.proc.oneshot():
            # only return time deltas
            # TODO[tahir]: better abstraction for metrics based on last value
            cpu_time_sys_total = self.proc.cpu_times().system
            cpu_time_user_total = self.proc.cpu_times().user
            cpu_time_sys = cpu_time_sys_total - self.stored_value["CPU_TIME_SYS_TOTAL"]
            cpu_time_user = cpu_time_user_total - self.stored_value["CPU_TIME_USER_TOTAL"]

            ctx_switch_voluntary_total = self.proc.num_ctx_switches().voluntary
            ctx_switch_involuntary_total = self.proc.num_ctx_switches().involuntary
            ctx_switch_voluntary = ctx_switch_voluntary_total - self.stored_value["CTX_SWITCH_VOLUNTARY_TOTAL"]
            ctx_switch_involuntary = ctx_switch_involuntary_total - self.stored_value["CTX_SWITCH_INVOLUNTARY_TOTAL"]

            self.stored_value = dict(
                CPU_TIME_SYS_TOTAL=cpu_time_sys_total,
                CPU_TIME_USER_TOTAL=cpu_time_user_total,
                CTX_SWITCH_VOLUNTARY_TOTAL=ctx_switch_voluntary_total,
                CTX_SWITCH_INVOLUNTARY_TOTAL=ctx_switch_involuntary_total,
            )

            metrics = [
                (THREAD_COUNT, self.proc.num_threads()),
                (MEM_RSS, self.proc.memory_info().rss),
                (CTX_SWITCH_VOLUNTARY, ctx_switch_voluntary),
                (CTX_SWITCH_INVOLUNTARY, ctx_switch_involuntary),
                (CPU_TIME_SYS, cpu_time_sys),
                (CPU_TIME_USER, cpu_time_user),
                (CPU_PERCENT, self.proc.cpu_percent()),
            ]

            return metrics
