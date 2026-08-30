import {
  DndContext,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useState } from "react";

type Card = { id: string; title: string };

function SortableCard({ card }: { card: Card }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: card.id,
  });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.7 : 1,
  };
  return (
    <li
      ref={setNodeRef}
      style={style}
      className="run-tool-pipeline-card"
      data-testid={`run-pipeline-card-${card.id}`}
      {...attributes}
      {...listeners}
    >
      {card.title}
    </li>
  );
}

function asCards(raw: unknown): Card[] {
  if (!Array.isArray(raw)) return [];
  return raw
    .map((item, i) => {
      if (!item || typeof item !== "object") return null;
      const o = item as Record<string, unknown>;
      const title =
        typeof o.title === "string"
          ? o.title
          : typeof o.Name === "string"
            ? o.Name
            : `Card ${i + 1}`;
      const id = typeof o.id === "string" ? o.id : `card-${i}-${title}`;
      return { id, title };
    })
    .filter((c): c is Card => c != null);
}

/** Pipeline lane with dnd-kit sortable cards (local reorder only in Phase 3). */
export function PipelineLaneNode({
  title,
  stage,
  cards: cardsRaw,
}: {
  title?: string;
  stage?: string;
  cards: unknown;
}) {
  const [cards, setCards] = useState(() => asCards(cardsRaw));
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    setCards((prev) => {
      const oldIndex = prev.findIndex((c) => c.id === active.id);
      const newIndex = prev.findIndex((c) => c.id === over.id);
      if (oldIndex < 0 || newIndex < 0) return prev;
      return arrayMove(prev, oldIndex, newIndex);
    });
  };

  return (
    <div className="run-tool-pipeline-lane" data-testid="run-pipeline-lane">
      <header className="canvas-node-header">
        <h4 className="canvas-node-title">{title ?? stage ?? "Stage"}</h4>
        <p className="muted">{cards.length} card(s)</p>
      </header>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <SortableContext items={cards.map((c) => c.id)} strategy={verticalListSortingStrategy}>
          <ul className="canvas-pipeline-cards run-tool-pipeline-cards">
            {cards.length === 0 ? (
              <li className="muted">No cards</li>
            ) : (
              cards.map((card) => <SortableCard key={card.id} card={card} />)
            )}
          </ul>
        </SortableContext>
      </DndContext>
    </div>
  );
}
