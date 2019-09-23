import React from 'react';
import Pairing from './pairing';
import Dispenser from './dispenser';
import { useDispenserState } from './hooks/state';

function App() {
  const [dispenser, setDispenser] = useDispenserState(null);

  function onPair(pairedDispenser) {
    setDispenser(pairedDispenser);
  }

  function onUnpair(pairedDispenser) {
    setDispenser(null);
  }

  return (
    <div className="container">
      {dispenser ? (
        <Dispenser onUnpair={onUnpair} />
      ) : (
        <Pairing onPaired={onPair} />
      )}
      <style jsx global>{`
        body {
          background-color: #f2f2f2;
          margin: 0;
          padding: 80px 0;
          font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Roboto", "Oxygen", "Ubuntu", "Cantarell", "Fira Sans", "Droid Sans", "Helvetica Neue", sans-serif;
          -webkit-font-smoothing: antialiased;
          -moz-osx-font-smoothing: grayscale;
        }

        code {
          font-family: source-code-pro, Menlo, Monaco, Consolas, "Courier New", monospace;
        }
      `}</style>
    </div>
  );
}

export default App;
