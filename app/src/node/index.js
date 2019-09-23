import React from 'react';

function Node({
  name,
  onRename,
  status,
  onUnlock,
}) {
  return (
    <article>
      <h1>{name}</h1>
      <p>
        <button onClick={onUnlock}>{status}</button>
      </p>
      <style jsx>{`
        article {
          display: block;
          background: white;
          padding: 10px;
        }
      `}</style>
    </article>
  );
}

export default Node;
