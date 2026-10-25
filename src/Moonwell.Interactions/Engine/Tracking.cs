using System;
using System.Collections.Generic;
using Moonwell.Hosting;

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

        /// <summary>Ends the cycle's timer, and forgets the cycle, the entries and what was asked for; Interactions.Install calls it, as an install starts the package afresh.</summary>
        public static void Reset()
        {
            if (timer != null)
            {
                timer.End();
                timer = null;
            }

            cycle = null;
            checks = null;
            scan = null;
            tracked = 0;
            lastId = 0;
            unitsAsked = false;
            itemsAsked = false;
            destructablesAsked = false;
            unitEntries.Clear();
            itemEntries.Clear();
            destructableEntries.Clear();
        }

        /// <summary>Asks for a kind of thing to be tracked. The first ask for a kind fills it with the things there are: the live units, the items and destructables of the world, taken in if the runtime has not. Those made after come in through core's take-in hook. Gizmos are always tracked. The package installs itself on the game first if it is not installed, as an install after the ask would forget it.</summary>
        public static void Need(int kind)
        {
            Interactions.EnsureInstalled();
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

        /// <summary>Asks for the kinds of thing a class covers: Unit and its subclasses units, Item items, Destructable destructables, Gizmo and a map's gizmo classes gizmos, Widget the first three and Actor all four. Any other class has no place, which fails in a development build. The package installs itself on the game first if it is not installed.</summary>
        public static void NeedFor(Type type)
        {
            Interactions.EnsureInstalled();
            for (var kind = UnitKind; kind <= DestructableKind; kind++)
            {
                if (Covers(type, kind))
                {
                    Need(kind);
                }
            }

            CheckPlace(type);
        }

        /// <summary>Fails in a development build when a class covers no kind of thing, as Player and the game do not have a place.</summary>
        public static void CheckPlace(Type type)
        {
            Dev.Check(Covers(type, UnitKind) || Covers(type, ItemKind) || Covers(type, DestructableKind) || Covers(type, GizmoKind), type.Name + " has no place");
        }

        /// <summary>Whether a class covers a kind of thing: Unit and its subclasses units, Item items, Destructable destructables, Gizmo and a map's gizmo classes gizmos, Widget the first three and Actor all four.</summary>
        public static bool Covers(Type type, int kind)
        {
            if (type == typeof(Actor))
            {
                return true;
            }

            if (kind == UnitKind)
            {
                return type == typeof(Widget) || typeof(Unit).IsAssignableFrom(type);
            }

            if (kind == ItemKind)
            {
                return type == typeof(Widget) || typeof(Item).IsAssignableFrom(type);
            }

            if (kind == DestructableKind)
            {
                return type == typeof(Widget) || typeof(Destructable).IsAssignableFrom(type);
            }

            return typeof(Gizmo).IsAssignableFrom(type);
        }

        /// <summary>Whether a kind of thing is tracked: it has been asked for while the package is installed. Gizmos always are.</summary>
        public static bool Asked(int kind)
        {
            if (kind == GizmoKind)
            {
                return true;
            }

            if (!Interactions.IsInstalled)
            {
                return false;
            }

            if (kind == UnitKind)
            {
                return unitsAsked;
            }

            return kind == ItemKind ? itemsAsked : destructablesAsked;
        }

        /// <summary>Adds, to the end of a list, an entry that is in no grid for each live thing of class T of a kind nobody has asked for, in the order the runtime took them in. It tracks nothing and keeps nothing: the entries are the search's own, to be dropped with it. For a search inside Local.For, which may not track a kind.</summary>
        public static void AddUntracked<T>(List<Entry> into)
            where T : Actor
        {
            var type = typeof(T);
            if (Covers(type, UnitKind) && !Asked(UnitKind))
            {
                var live = KnownUnits.Live;
                for (var i = 0; i < live.Count; i++)
                {
                    var unit = live[i];
                    if (unit is T && !unit.IsGone && !unit.HasEnded && unit.Handle != null)
                    {
                        into.Add(new Entry(0, unit, UnitKind));
                    }
                }
            }

            if (Covers(type, ItemKind) && !Asked(ItemKind))
            {
                var live = KnownItems.Live;
                for (var i = 0; i < live.Count; i++)
                {
                    var item = live[i];
                    if (item is T && !item.IsGone && !item.HasEnded && item.Handle != null)
                    {
                        into.Add(new Entry(0, item, ItemKind));
                    }
                }
            }

            if (Covers(type, DestructableKind) && !Asked(DestructableKind))
            {
                var live = KnownDestructables.Live;
                for (var i = 0; i < live.Count; i++)
                {
                    var destructable = live[i];
                    if (destructable is T && !destructable.IsGone && !destructable.HasEnded)
                    {
                        var entry = new Entry(0, destructable, DestructableKind);
                        entry.X = destructable.X;
                        entry.Y = destructable.Y;
                        into.Add(entry);
                    }
                }
            }
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

        /// <summary>Reads the place of an entry without keeping it, for a search inside Local.For, which may change no shared state: whether it has a place and, if it has, where, through out. An entry read this step gives what that read kept; any other is read from the game into locals, and its cache, its cell and the grid are left as they were. The same rule as ReadNow decides what has a place.</summary>
        public static bool Peek(Entry entry, out float x, out float y)
        {
            x = entry.X;
            y = entry.Y;
            if (entry.Ended)
            {
                return false;
            }

            if (entry.Kind == GizmoKind)
            {
                return entry.HasPlace;
            }

            if (cycle != null && cycle.Running && entry.ReadStep == cycle.StepNow)
            {
                return entry.HasPlace;
            }

            var world = Interactions.PeekWorld;
            if (entry.Kind == UnitKind)
            {
                var unit = (Unit)entry.Thing;
                if (!UnitHasPlace(unit, world))
                {
                    return false;
                }

                x = unit.X;
                y = unit.Y;
                return true;
            }

            if (entry.Kind == ItemKind)
            {
                var item = (Item)entry.Thing;
                if (!ItemHasPlace(item, world))
                {
                    return false;
                }

                x = item.X;
                y = item.Y;
                return true;
            }

            var destructable = (Destructable)entry.Thing;
            return !destructable.IsGone && world.IsAlive(destructable.Handle);
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

        /// <summary>Tells the package a thing is ending, before its own reaction runs: it has no place from now, the interactions it holds end, newest first, and then the pairs whose other it is part, newest first, so every OnPart runs before the thing's OnLeave. It is still in the grid.</summary>
        public static void Ending(Entry entry)
        {
            entry.Ended = true;
            entry.HasPlace = false;
            var held = entry.Held;
            if (held != null)
            {
                while (held.Count > 0)
                {
                    var interaction = held[held.Count - 1];
                    held.RemoveAt(held.Count - 1);
                    interaction.End();
                }
            }

            var partners = entry.Partners;
            if (partners != null)
            {
                while (partners.Count > 0)
                {
                    var pair = partners[partners.Count - 1];
                    partners.RemoveAt(partners.Count - 1);
                    pair.End();
                }
            }
        }

        /// <summary>Notes that a thing holds an interaction, so the interaction ends when the thing does.</summary>
        public static void Hold(Entry entry, Interaction interaction)
        {
            if (entry.Held == null)
            {
                entry.Held = new List<Interaction>();
            }

            entry.Held.Add(interaction);
        }

        /// <summary>Forgets that a thing holds an interaction that is ending; one it no longer holds is passed over.</summary>
        public static void Unhold(Entry entry, Interaction interaction)
        {
            var held = entry.Held;
            if (held == null)
            {
                return;
            }

            for (var i = held.Count - 1; i >= 0; i--)
            {
                if (held[i] == interaction)
                {
                    held.RemoveAt(i);
                    return;
                }
            }
        }

        /// <summary>Notes that a thing is the other of a pair, so the pair parts when the thing ends.</summary>
        public static void AddPartner(Entry entry, Owner pair)
        {
            if (entry.Partners == null)
            {
                entry.Partners = new List<Owner>();
            }

            entry.Partners.Add(pair);
        }

        /// <summary>Forgets that a thing is the other of a pair that is parting; one it is not the other of is passed over.</summary>
        public static void RemovePartner(Entry entry, Owner pair)
        {
            var partners = entry.Partners;
            if (partners == null)
            {
                return;
            }

            for (var i = partners.Count - 1; i >= 0; i--)
            {
                if (partners[i] == pair)
                {
                    partners.RemoveAt(i);
                    return;
                }
            }
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

        /// <summary>Tracks a unit, item or destructable of a kind asked for that the take-in hook has not brought in yet: one that starts an interaction in its OnCreated or a start component, which run before the runtime calls the hook. The hook after finds it tracked and passes it over. One that is ending, or a gizmo, is passed over.</summary>
        public static void TakeInEarly(Actor thing)
        {
            var widget = thing as Widget;
            if (widget != null && !widget.ending && !widget.HasEnded)
            {
                TakenIn(widget);
            }
        }

        /// <summary>Core's hook for a widget ending: it leaves the grid and the cell checks at once, before its OnLeave runs. A widget the package never saw, such as one taken in before it was installed or one of a kind nobody asked for, is passed over.</summary>
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
            return UnitHasPlace(unit, Interactions.World);
        }

        /// <summary>Whether a unit has a place, asking a world.</summary>
        private static bool UnitHasPlace(Unit unit, IInteractionWorld world)
        {
            return unit.IsAlive && !world.IsHidden(unit.Handle) && !world.IsLoaded(unit.Handle);
        }

        /// <summary>Whether an item has a place: on the ground, so not carried, and visible.</summary>
        private static bool ItemHasPlace(Item item)
        {
            return ItemHasPlace(item, Interactions.World);
        }

        /// <summary>Whether an item has a place, asking a world.</summary>
        private static bool ItemHasPlace(Item item, IInteractionWorld world)
        {
            return !item.IsGone && !item.IsCarried && world.IsVisible(item.Handle);
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
