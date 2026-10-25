using System;
using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>What the package tracks: the entries it gives to things with a place, and the count of them that decides whether the cycle runs. Gizmos are always tracked; units, items and destructables are tracked once something asks for that kind, so a map that only finds gizmos tracks none of the game's things.</summary>
    internal static class Tracking
    {
        /// <summary>The kind of a unit.</summary>
        public const int UnitKind = 1;

        /// <summary>The kind of an item.</summary>
        public const int ItemKind = 2;

        /// <summary>The kind of a destructable.</summary>
        public const int DestructableKind = 3;

        /// <summary>The kind of a gizmo.</summary>
        public const int GizmoKind = 4;

        /// <summary>The entry of each unit tracked, keyed by the game's handle; a lookup only.</summary>
        private static readonly Dictionary<unit, Entry> unitEntries = new Dictionary<unit, Entry>();

        /// <summary>The entry of each item tracked, keyed by the game's handle; a lookup only.</summary>
        private static readonly Dictionary<item, Entry> itemEntries = new Dictionary<item, Entry>();

        /// <summary>The entry of each destructable tracked, keyed by the game's handle; a lookup only.</summary>
        private static readonly Dictionary<destructable, Entry> destructableEntries = new Dictionary<destructable, Entry>();

        /// <summary>The cycle; made on first use after an Install.</summary>
        private static Cycle? cycle;

        /// <summary>The cell checks of the units and items; made on first use after an Install.</summary>
        private static CellChecks? checks;

        /// <summary>The scan for items made with no Moonwell code; made when items are first asked for.</summary>
        private static ItemScan? scan;

        /// <summary>The timer that runs the cycle every step; null while nothing needs it.</summary>
        private static Timer? timer;

        /// <summary>How many things are tracked.</summary>
        private static int tracked;

        /// <summary>The id the last entry was given.</summary>
        private static int lastId;

        /// <summary>Whether units have been asked for.</summary>
        private static bool unitsAsked;

        /// <summary>Whether items have been asked for.</summary>
        private static bool itemsAsked;

        /// <summary>Whether destructables have been asked for.</summary>
        private static bool destructablesAsked;

        /// <summary>The cycle, made on first use.</summary>
        public static Cycle Beat
        {
            get
            {
                if (cycle == null)
                {
                    cycle = new Cycle();
                }

                return cycle;
            }
        }

        /// <summary>The step the cycle is at.</summary>
        public static int StepNow
        {
            get { return Beat.StepNow; }
        }

        /// <summary>Whether the cycle's timer is running.</summary>
        public static bool Running
        {
            get { return timer != null && !timer.HasEnded; }
        }

        /// <summary>How many things are tracked.</summary>
        public static int Count
        {
            get { return tracked; }
        }

        /// <summary>The cell checks, made on first use.</summary>
        private static CellChecks Checks
        {
            get
            {
                if (checks == null)
                {
                    checks = new CellChecks();
                }

                return checks;
            }
        }

        /// <summary>Forgets the cycle, the entries and what was asked for; Interactions.Install calls it, as an install starts the package afresh.</summary>
        public static void Reset()
        {
            cycle = null;
            checks = null;
            scan = null;
            timer = null;
            tracked = 0;
            lastId = 0;
            unitsAsked = false;
            itemsAsked = false;
            destructablesAsked = false;
            unitEntries.Clear();
            itemEntries.Clear();
            destructableEntries.Clear();
        }

        /// <summary>Asks for a kind of thing to be tracked. The first ask for a kind fills it with the things there are: the live units, the items and destructables of the world, taken in if the runtime has not. Those made after come in through core's take-in hook. Gizmos are always tracked.</summary>
        public static void Need(int kind)
        {
            if (kind == UnitKind)
            {
                if (!unitsAsked)
                {
                    unitsAsked = true;
                    for (var i = 0; i < KnownUnits.Live.Count; i++)
                    {
                        AddUnit(KnownUnits.Live[i]);
                    }
                }
            }
            else if (kind == ItemKind)
            {
                if (!itemsAsked)
                {
                    itemsAsked = true;
                    StartTimer();
                    Scan.Arm();
                    TrackItemsInWorld();
                }
            }
            else if (kind == DestructableKind)
            {
                if (!destructablesAsked)
                {
                    destructablesAsked = true;
                    var handles = Runtime.World.DestructablesInWorld();
                    for (var i = 0; i < handles.Count; i++)
                    {
                        var found = Destructable.Of(handles[i]);
                        if (found != null)
                        {
                            AddDestructable(found);
                        }
                    }
                }
            }
            else
            {
                Dev.Check(kind == GizmoKind, "Tracking.Need: " + kind + " is not a kind of thing");
            }
        }

        /// <summary>Asks for the kinds of thing a class covers: Unit and its subclasses units, Item items, Destructable destructables, Gizmo and a map's gizmo classes gizmos, Widget the first three and Actor all four. Any other class has no place, which fails in a development build.</summary>
        public static void NeedFor(Type type)
        {
            var widget = type == typeof(Widget);
            var all = type == typeof(Actor);
            var any = false;
            if (all || widget || typeof(Unit).IsAssignableFrom(type))
            {
                Need(UnitKind);
                any = true;
            }

            if (all || widget || typeof(Item).IsAssignableFrom(type))
            {
                Need(ItemKind);
                any = true;
            }

            if (all || widget || typeof(Destructable).IsAssignableFrom(type))
            {
                Need(DestructableKind);
                any = true;
            }

            if (all || typeof(Gizmo).IsAssignableFrom(type))
            {
                any = true;
            }

            Dev.Check(any, type.Name + " has no place");
        }

        /// <summary>The entry of a thing; null when it is not tracked, has ended, or is a default object with no handle.</summary>
        public static Entry? EntryOf(Actor thing)
        {
            Entry? entry = null;
            var gizmo = thing as Gizmo;
            if (gizmo != null)
            {
                entry = gizmo.entry;
            }
            else
            {
                var unit = thing as Unit;
                var item = thing as Item;
                var destructable = thing as Destructable;
                if (unit != null && unit.Handle != null)
                {
                    unitEntries.TryGetValue(unit.Handle, out entry);
                }
                else if (item != null && item.Handle != null)
                {
                    itemEntries.TryGetValue(item.Handle, out entry);
                }
                else if (destructable != null && destructable.Handle != null)
                {
                    destructableEntries.TryGetValue(destructable.Handle, out entry);
                }
            }

            return entry != null && !entry.Ended ? entry : null;
        }

        /// <summary>Reads the place of an entry, at most once a step: whether it has a place, and where, and its cell follows. A unit has a place when it is alive, not hidden and not loaded; an item when it is not carried and is visible; a destructable when it is alive, and it is not read for where, as it does not move; a gizmo always, until it ends. An entry that has ended is left alone. Outside a running step, as when a button asks, there is no step to read once in: every ask reads. Inside one, a thing that moves after it was read is seen at its old place until the next step.</summary>
        public static void ReadNow(Entry entry)
        {
            if (entry.Ended)
            {
                return;
            }

            var beat = Beat;
            if (beat.Running)
            {
                if (entry.ReadStep == beat.StepNow)
                {
                    return;
                }

                entry.ReadStep = beat.StepNow;
            }

            if (entry.Kind == UnitKind)
            {
                ReadUnit(entry, (Unit)entry.Thing);
            }
            else if (entry.Kind == ItemKind)
            {
                ReadItem(entry, (Item)entry.Thing);
            }
            else if (entry.Kind == DestructableKind)
            {
                ReadDestructable(entry, (Destructable)entry.Thing);
            }
        }

        /// <summary>Gives a thing its entry: the next id and its place, in the grid and with a place. The first thing tracked starts the cycle's timer.</summary>
        public static Entry Begin(Actor thing, int kind, float x, float y)
        {
            var grid = Interactions.Grid;
            lastId++;
            var entry = new Entry(lastId, thing, kind);
            entry.X = x;
            entry.Y = y;
            entry.HasPlace = true;
            grid.Add(entry);
            tracked++;
            StartTimer();
            return entry;
        }

        /// <summary>Tells the package a thing is ending, before its own reaction runs: it has no place from now. It is still in the grid.</summary>
        public static void Ending(Entry entry)
        {
            entry.Ended = true;
            entry.HasPlace = false;
        }

        /// <summary>Takes an ending thing out of the grid and stops tracking it. The last one tracked ends the cycle's timer, unless items are asked for.</summary>
        public static void Release(Entry entry)
        {
            Interactions.Grid.Remove(entry);
            if (entry.Kind == UnitKind)
            {
                unitEntries.Remove(((Unit)entry.Thing).Handle);
                Checks.Remove(entry);
            }
            else if (entry.Kind == ItemKind)
            {
                itemEntries.Remove(((Item)entry.Thing).Handle);
                Checks.Remove(entry);
            }
            else if (entry.Kind == DestructableKind)
            {
                destructableEntries.Remove(((Destructable)entry.Thing).Handle);
            }

            if (tracked > 0)
            {
                tracked--;
            }

            if (tracked == 0 && !itemsAsked && timer != null)
            {
                timer.End();
                timer = null;
            }
        }

        /// <summary>Core's hook for a widget taken in: tracks it when its kind has been asked for.</summary>
        public static void TakenIn(Widget widget)
        {
            var unit = widget as Unit;
            var item = widget as Item;
            var destructable = widget as Destructable;
            if (unit != null)
            {
                if (unitsAsked)
                {
                    AddUnit(unit);
                }
            }
            else if (item != null)
            {
                if (itemsAsked)
                {
                    AddItem(item);
                }
            }
            else if (destructable != null && destructablesAsked)
            {
                AddDestructable(destructable);
            }
        }

        /// <summary>Core's hook for a widget ending: it leaves the grid and the cell checks at once. A widget the package never saw, such as one taken in before it was installed or one of a kind nobody asked for, is passed over.</summary>
        public static void WidgetEnding(Widget widget)
        {
            var entry = EntryOf(widget);
            if (entry != null)
            {
                Ending(entry);
                Release(entry);
            }
        }

        /// <summary>Takes in and tracks the items of the world that are not tracked yet. The cycle does it once a second: the game makes items with no Moonwell code, a creep's drop for one. An item already tracked is passed over by a lookup of its handle.</summary>
        public static void TrackItemsInWorld()
        {
            var handles = Runtime.World.ItemsInWorld();
            for (var i = 0; i < handles.Count; i++)
            {
                if (itemEntries.ContainsKey(handles[i]))
                {
                    continue;
                }

                var found = Item.Of(handles[i]);
                if (found != null)
                {
                    AddItem(found);
                }
            }
        }

        /// <summary>The scan for items, made on first use.</summary>
        private static ItemScan Scan
        {
            get
            {
                if (scan == null)
                {
                    scan = new ItemScan();
                }

                return scan;
            }
        }

        /// <summary>Starts the cycle's timer unless it is running.</summary>
        private static void StartTimer()
        {
            if (!Running)
            {
                timer = Timer.Every(Interactions.Step, Run, Runtime.Root);
            }
        }

        /// <summary>Tracks a unit that is not tracked: its entry is in the grid at its place, and in the cell checks.</summary>
        private static void AddUnit(Unit unit)
        {
            if (unit.IsGone || unit.HasEnded || unit.Handle == null || unitEntries.ContainsKey(unit.Handle))
            {
                return;
            }

            var entry = Begin(unit, UnitKind, unit.X, unit.Y);
            entry.ReadStep = StepNow;
            unitEntries[unit.Handle] = entry;
            entry.HasPlace = UnitHasPlace(unit);
            Checks.Add(entry);
        }

        /// <summary>Tracks an item that is not tracked.</summary>
        private static void AddItem(Item item)
        {
            if (item.IsGone || item.HasEnded || item.Handle == null || itemEntries.ContainsKey(item.Handle))
            {
                return;
            }

            var entry = Begin(item, ItemKind, item.X, item.Y);
            entry.ReadStep = StepNow;
            itemEntries[item.Handle] = entry;
            entry.HasPlace = ItemHasPlace(item);
            Checks.Add(entry);
        }

        /// <summary>Tracks a destructable that is not tracked, at the place it was taken in at; it is not in the cell checks.</summary>
        private static void AddDestructable(Destructable destructable)
        {
            if (destructable.IsGone || destructable.HasEnded || destructableEntries.ContainsKey(destructable.Handle))
            {
                return;
            }

            var entry = Begin(destructable, DestructableKind, destructable.X, destructable.Y);
            entry.ReadStep = StepNow;
            destructableEntries[destructable.Handle] = entry;
            entry.HasPlace = Interactions.World.IsAlive(destructable.Handle);
        }

        /// <summary>Whether a unit has a place: alive, not hidden and not loaded in a transport.</summary>
        private static bool UnitHasPlace(Unit unit)
        {
            var world = Interactions.World;
            return unit.IsAlive && !world.IsHidden(unit.Handle) && !world.IsLoaded(unit.Handle);
        }

        /// <summary>Whether an item has a place: on the ground, so not carried, and visible.</summary>
        private static bool ItemHasPlace(Item item)
        {
            return !item.IsGone && !item.IsCarried && Interactions.World.IsVisible(item.Handle);
        }

        /// <summary>Reads a unit: whether it has a place and, if it has, where. A unit with none keeps the place it was last read at.</summary>
        private static void ReadUnit(Entry entry, Unit unit)
        {
            entry.HasPlace = UnitHasPlace(unit);
            if (entry.HasPlace)
            {
                Interactions.Grid.Move(entry, unit.X, unit.Y);
            }
        }

        /// <summary>Reads an item: whether it has a place and, if it has, where.</summary>
        private static void ReadItem(Entry entry, Item item)
        {
            entry.HasPlace = ItemHasPlace(item);
            if (entry.HasPlace)
            {
                Interactions.Grid.Move(entry, item.X, item.Y);
            }
        }

        /// <summary>Reads a destructable: whether it is still alive. Where it is was read once.</summary>
        private static void ReadDestructable(Entry entry, Destructable destructable)
        {
            entry.HasPlace = !destructable.IsGone && Interactions.World.IsAlive(destructable.Handle);
        }

        /// <summary>The cycle's timer action: one step.</summary>
        private static void Run()
        {
            Beat.Run();
        }
    }
}
