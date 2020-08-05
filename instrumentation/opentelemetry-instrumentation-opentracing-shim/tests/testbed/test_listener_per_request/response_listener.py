class ResponseListener:
    def __init__(self, span):
        self.span = span

    def on_response(self, res):
        del res
        self.span.finish()
