import React, { useState } from 'react';
import { useModal } from 'react-modal-hook';
import Node from '../node';
import Modal from '../modal';
import { useNodesState } from '../hooks/state';

function Nodes() {
  const [nodes, setNodes] = useNodesState([]);
  const [count, setCount] = useState(0);

  const [showModal] = useModal(({ in: open, onExited }) => (
    <Modal open={open} onClose={onExited}>
      <span>The count is {count}</span>
      <button onClick={() => setCount(count + 1)}>Increment</button>
    </Modal>
  ), [count]);

  function rename() {
  }

  function unlock() {
    showModal();
  }

  return (
    <div>
      {nodes.map(node => (
        <Node />
      ))}
      <Node
        name="Coincenter"
        onRename={rename}
        status="locked"
        onUnlock={unlock}
      />
      <div className="">
        add new...
      </div>
      <style jsx>{`
        article {
          display: block;
          background: white;
          padding: 10px;
        }
      `}</style>
    </div>
  );
}

export default Nodes;
