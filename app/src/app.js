import React, { useEffect } from 'react';
import Dispenser from './dispenser';
import { useDispenserState } from './hooks/state';

function App() {
  const [dispenser, setDispenser] = useDispenserState(null);

  useEffect(() => {
    async function doFetch() {
      const res = await fetch('http://localhost:9000/api/v1/dispenser');
      const dispenser = await res.json();
      setDispenser(dispenser);
    }

    doFetch();
  }, []);

  return (
    <div className="container">
      {dispenser && (
        <h1>{dispenser.name}</h1>
      )}
      <Dispenser />
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
