using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>What the package keeps of one thing that has a place: a unit, an item, a destructable or a gizmo. The grid and the cycle read these fields directly; a map never sees an entry.</summary>
    internal sealed class Entry
    {
        /// <summary>The entry's number, in the order entries were made; it is how a list of entries is searched without keying by a class.</summary>
        public int Id;

        /// <summary>The thing the entry stands for.</summary>
        public Actor Thing;

        /// <summary>What kind of thing it is: Tracking.UnitKind, ItemKind, DestructableKind or GizmoKind.</summary>
        public int Kind;

        /// <summary>The place the entry was last read at, left to right.</summary>
        public float X;

        /// <summary>The place the entry was last read at, bottom to top.</summary>
        public float Y;

        /// <summary>The grid cell the entry is in; -1 when it is in none.</summary>
        public int Cell = -1;

        /// <summary>The entry's index in the cell checks' list; -1 when it is in none, as a destructable's or a gizmo's.</summary>
        public int Ring = -1;

        /// <summary>The step of the cycle the place was last read at; -1 before it was read.</summary>
        public int ReadStep = -1;

        /// <summary>Whether the thing has a place now: a unit that is alive, not hidden and not loaded; an item on the ground; a destructable that is alive; a gizmo that has not begun to end, so not in its OnLeave.</summary>
        public bool HasPlace;

        /// <summary>Whether the thing has begun to end: it has no place from then on, and no read gives it one again.</summary>
        public bool Ended;

        /// <summary>The interactions the thing holds that have not ended, in the order they started; null until it holds one.</summary>
        public List<Interaction>? Held;

        /// <summary>The pairs whose other is the thing that have not parted, in the order they met; null until it is the other of one.</summary>
        public List<Owner>? Partners;

        /// <summary>Makes the entry of a thing, in no cell, with no place read.</summary>
        public Entry(int id, Actor thing, int kind)
        {
            Id = id;
            Thing = thing;
            Kind = kind;
        }
    }
}
