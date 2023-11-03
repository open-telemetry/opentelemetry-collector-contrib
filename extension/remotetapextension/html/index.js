// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

const tapSelect = document.getElementById("tap");

const xmlHttp = new XMLHttpRequest();
xmlHttp.open("GET", "/taps", false);
xmlHttp.send(null);

const taps = JSON.parse(xmlHttp.responseText);
const emptyOpt = document.createElement('option');
emptyOpt.value = '';
emptyOpt.innerHTML = "&lt;none&gt;";
tapSelect.appendChild(emptyOpt);
for (tap of taps) {
    const opt = document.createElement('option');
    opt.value = tap.endpoint;
    opt.innerHTML = tap.name;
    tapSelect.appendChild(opt);
}

tapSelect.oninput = function (e) {
    const textarea = document.getElementById("data");
    textarea.value = '';
    if (tapSelect.value !== '') {
        connectTo(tapSelect.value);
    }
}

function connectTo(endpoint) {
    const socket = new WebSocket('ws://' + endpoint);
    socket.addEventListener('open', function (event) {
        socket.send('Server connected to ' + socket);
    });

    socket.addEventListener('message', function (event) {
        const textarea = document.getElementById("data");
        textarea.value += event.data;
    });
}