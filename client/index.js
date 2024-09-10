async function main() {
  const app = document.getElementById("app");
  if (!app) return;

  const ws = new WebSocket("ws://localhost:3000/ws");
  const messages = [];

  // play with inspect
  window.ws = ws;
  window.messages = messages;

  ws.onopen = (ev) => {
    console.log({ open: ev });
  };

  ws.onclose = (ev) => {
    console.log({ close: ev });
  };

  ws.onerror = (ev) => {
    console.log({ error: ev });
  };

  ws.onmessage = (ev) => {
    console.log("Message length", ev.data.length);
    console.log({ message: ev.data });
    messages.push(ev.data);
  };
}

main();
