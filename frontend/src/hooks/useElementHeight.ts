import { useEffect, useRef, useState } from 'react';

export function useElementHeight<T extends HTMLElement>() {
  const ref = useRef<T | null>(null);
  const [height, setHeight] = useState(0);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;

    const updateHeight = () => {
      setHeight(Math.max(0, Math.floor(node.getBoundingClientRect().height)));
    };

    updateHeight();

    const observer = new ResizeObserver(() => {
      updateHeight();
    });
    observer.observe(node);

    return () => {
      observer.disconnect();
    };
  }, []);

  return [ref, height] as const;
}
