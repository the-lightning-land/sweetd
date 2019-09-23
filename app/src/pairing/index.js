import React from 'react';
import { useBluetooth } from '../hooks/bluetooth';

function Pairing({ onPaired }) {
  const {
    supported,
    connecting,
    connect,
    disconnect,
    dispenser,
    availableWifis,
    forceUpdate,
    scanWifi,
    connectWifi,
  } = useBluetooth();

  function wifi(event) {
    connectWifi('ssid', 'psk');
  }

  function finish() {
    onPaired({});
  }

  return (
    <div className="container">
      <h1>
        You've got your <a href="https://the.lightning.land/candy">Bitcoin Candy Dispenser</a>?
      </h1>
      <p>Let's set it up then <span role="img" aria-label="yay!">🎉</span></p>

      {supported() ? (
        <div>
          <div>{connecting && 'connecting...'}</div>
          {dispenser && (
            <div>{JSON.stringify(dispenser)}</div>
          )}
          {availableWifis && (
            <ul>
              {availableWifis.map(wifi => (
                <li>{wifi}</li>
              ))}
            </ul>
          )}
          <button onClick={connect}>Connect</button>
          <button onClick={forceUpdate}>Force</button>
          <button onClick={disconnect}>Disconnect</button>

          <button onClick={scanWifi}>Scan Wifi</button>
          <button onClick={wifi}>Connect Wifi</button>
          <button onClick={finish}>Finish</button>
        </div>
      ) : (
        <div>sorry, not supported</div>
      )}
      <style jsx>{`
        .container {
          max-width: 768px;
          margin: 0 auto;
          padding: 0 20px;
          width: 100%;
        }

        button {
          padding: 20px;
        }
      `}</style>
    </div>
  );
}

export default Pairing;
