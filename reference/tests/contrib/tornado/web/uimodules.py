import tornado


class Item(tornado.web.UIModule):
    def render(self, item):
        return self.render_string('templates/item.html', item=item)
