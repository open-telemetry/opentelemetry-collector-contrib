from abc import ABCMeta, abstractmethod

# ref: https://stackoverflow.com/a/38668373
ABC = ABCMeta('ABC', (object,), {'__slots__': ()})


class Propagator(ABC):

    @abstractmethod
    def inject(self, span_context, carrier):
        pass

    @abstractmethod
    def extract(self, carrier):
        pass
