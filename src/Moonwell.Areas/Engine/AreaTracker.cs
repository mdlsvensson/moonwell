using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>
    /// The package's one tracker: the areas in the order they were added, the game's death and revive events and core's
    /// ending hook (set up when the first area is added), and two timers. The step runs every area's OnUnitStays every
    /// 1/32 second while an area with a Tick has a unit inside; the check runs every area's place check every quarter of a
    /// second while an area has a unit inside, away (hidden, loaded or dead) or refused. Each ends when no area needs it,
    /// so a corpse or an area without a Tick keeps only the check running.
    /// </summary>
    internal sealed class AreaTracker
    {
        /// <summary>The seconds of a step of the package timer.</summary>
        internal const float Interval = 0.03125f;

        /// <summary>The seconds between place checks.</summary>
        internal const float CheckInterval = 0.25f;

        /// <summary>The tracker of the world and game now; made at the first area added after an install or a runtime reset.</summary>
        private static AreaTracker? current;

        /// <summary>The world this tracker talks to.</summary>
        internal readonly IAreaWorld World;

        /// <summary>The game this tracker was made for; a runtime reset makes another, and the tracker is stale.</summary>
        private readonly Game game;

        /// <summary>The areas in the order they were added.</summary>
        private readonly List<Area> areas = new List<Area>();

        /// <summary>The step timer; null while no area with a Tick has a unit inside.</summary>
        private Timer? step;

        /// <summary>The check timer; null while no area has a unit inside, away or refused.</summary>
        private Timer? check;

        /// <summary>How many steps the running step timer has made.</summary>
        internal int Steps;

        /// <summary>How many areas have a unit inside, away or refused.</summary>
        private int activeAreas;

        /// <summary>How many areas with a Tick have a unit inside.</summary>
        private int stayingAreas;

        /// <summary>Makes a tracker for a world and a game.</summary>
        private AreaTracker(IAreaWorld world, Game game)
        {
            World = world;
            this.game = game;
        }

        /// <summary>Whether a package timer, the step or the check, is running now; for the cases.</summary>
        internal static bool Running
        {
            get { return current != null && (current.step != null || current.check != null); }
        }

        /// <summary>Whether the 1/32 s step is running now; for the cases.</summary>
        internal static bool Stepping
        {
            get { return current != null && current.step != null; }
        }

        /// <summary>The tracker for the installed world and this game, made, with its game listeners, when there is none: a new install or a runtime reset makes a fresh one.</summary>
        internal static AreaTracker Current()
        {
            var world = AreaSystem.World;
            if (current == null || current.game != Game.Instance || current.World != world)
            {
                var made = new AreaTracker(world, Game.Instance);
                world.ListenDeaths(made.Died, made.Revived);
                Hooks.AddEnding(made.Ending);
                current = made;
#if DEBUG
                // The package's view in the debug mode, added once: Runtime.Reset clears the views, and with them the tracker.
                DebugViews.Add(AreasView.Instance);
#endif
            }

            return current;
        }

#if DEBUG
        /// <summary>The tracker of the installed world and this game when one has been made; null otherwise. Asking makes none.</summary>
        internal static AreaTracker? Existing()
        {
            if (current == null || current.game != Game.Instance || !object.ReferenceEquals(current.World, AreaSystem.WorldIfInstalled))
            {
                return null;
            }

            return current;
        }

        /// <summary>How many areas are being tracked.</summary>
        internal int AreaCount
        {
            get { return areas.Count; }
        }

        /// <summary>The area at a place in the order they were added.</summary>
        internal Area AreaAt(int index)
        {
            return areas[index];
        }
#endif

        /// <summary>Forgets the tracker, so that the next area starts a new one; for the cases.</summary>
        internal static void Forget()
        {
            current = null;
        }

        /// <summary>The package steps a Tick is: rounded up to whole 1/32 seconds; 0 for none.</summary>
        internal static int StepsOf(float tick)
        {
            if (!(tick > 0f))
            {
                return 0;
            }

            var steps = (int)MathF.Ceiling(tick / Interval - 0.001f);
            return steps < 1 ? 1 : steps;
        }

        /// <summary>Starts an area: makes its region, listens to it, and runs its first sweep after 0 seconds.</summary>
        internal void Begin(Area area, AreaShape shape)
        {
            var made = World.NewRegion();
            area.handle = made;
            AddParts(made, shape);
            World.Listen(made, area.HearEnter, area.HearLeave);
            areas.Add(area);
            Timer.After(0f, area.Sweep, area);
        }

        /// <summary>Stops an area: it is no longer stepped, and its region and triggers are destroyed.</summary>
        internal void Finish(Area area)
        {
            areas.Remove(area);
            if (area.handle != null)
            {
                World.DestroyRegion(area.handle);
                area.handle = null;
            }
        }

        /// <summary>An area starts or stops having a unit inside, away or refused: the first starts the check, the last ends it.</summary>
        internal void Activity(bool started)
        {
            if (started)
            {
                activeAreas++;
                if (activeAreas == 1)
                {
                    check = Timer.Every(CheckInterval, Check, game);
                }
            }
            else
            {
                activeAreas--;
                if (activeAreas == 0 && check != null)
                {
                    var ending = check;
                    check = null;
                    ending.End();
                }
            }
        }

        /// <summary>An area with a Tick starts or stops having a unit inside: the first starts the step, counting its steps from 0, and the last ends it.</summary>
        internal void Staying(bool started)
        {
            if (started)
            {
                stayingAreas++;
                if (stayingAreas == 1)
                {
                    Steps = 0;
                    step = Timer.Every(Interval, Step, game);
                }
            }
            else
            {
                stayingAreas--;
                if (stayingAreas == 0 && step != null)
                {
                    var ending = step;
                    step = null;
                    ending.End();
                }
            }
        }

        /// <summary>
        /// Adds the rects and the cells of a shape to a region: a rect or region part is a rect, a polygon or a circle part
        /// its 32 cells; a union adds those of its parts. A rect is given to the game from the middle of its first cell to
        /// the middle of its last, the cells it touches: the game gives a rect every cell from the one holding its min corner
        /// to the one holding its max corner, so a max edge on the 32 grid given as it is would take the cell past it, and
        /// two areas that share an edge would share a column of cells (seen 10-26).
        /// </summary>
        private void AddParts(region made, AreaShape shape)
        {
            if (shape.Kind == AreaShape.ShapeKind.Union)
            {
                for (var i = 0; i < shape.Parts.Length; i++)
                {
                    AddParts(made, shape.Parts[i]);
                }

                return;
            }

            if (shape.Kind == AreaShape.ShapeKind.Rect)
            {
                var bounds = shape.Bounds;
                World.AddRect(made, FirstMiddle(bounds.Min.X), FirstMiddle(bounds.Min.Y), LastMiddle(bounds.Min.X, bounds.Max.X), LastMiddle(bounds.Min.Y, bounds.Max.Y));
                return;
            }

            var centres = new List<float>();
            AreaCells.Of(shape, centres);
            for (var i = 0; i < centres.Count; i += 2)
            {
                World.AddCell(made, centres[i], centres[i + 1]);
            }
        }

        /// <summary>The middle of the 32 cell that holds a rect's min edge.</summary>
        private static float FirstMiddle(float min)
        {
            return MathF.Floor(min / 32f) * 32f + 16f;
        }

        /// <summary>The middle of the last 32 cell a rect from min to max reaches into: a max edge on the grid ends the cell before it. A rect with no width has the cell of its min edge.</summary>
        private static float LastMiddle(float min, float max)
        {
            var first = MathF.Floor(min / 32f);
            var last = MathF.Ceiling(max / 32f) - 1f;
            if (last < first)
            {
                last = first;
            }

            return last * 32f + 16f;
        }

        /// <summary>One step: each area whose Tick is due runs OnUnitStays, in the order the areas were added. A step stops when the step timer ends or restarts under it.</summary>
        private void Step()
        {
            var running = step;
            Steps++;
            var now = Steps;
            var inOrder = new List<Area>(areas);
            for (var i = 0; i < inOrder.Count; i++)
            {
                if (step != running)
                {
                    return;
                }

                inOrder[i].Stay(now);
            }
        }

        /// <summary>One check: each area's place check, in the order the areas were added. A check stops when the check timer ends or restarts under it.</summary>
        private void Check()
        {
            var running = check;
            var inOrder = new List<Area>(areas);
            for (var i = 0; i < inOrder.Count; i++)
            {
                if (check != running)
                {
                    return;
                }

                inOrder[i].CheckPlaces();
            }
        }

        /// <summary>The game says a unit died: it leaves every area it is in, in the order the areas were added, and stays among the units away of each while it is in the region.</summary>
        private void Died()
        {
            var unit = KnownUnits.Find(World.EventUnit());
            if (unit == null)
            {
                return;
            }

            var inOrder = new List<Area>(areas);
            for (var i = 0; i < inOrder.Count; i++)
            {
                inOrder[i].Die(unit);
            }
        }

        /// <summary>The game says a hero came back to life: it enters each area it is inside, in the order the areas were added.</summary>
        private void Revived()
        {
            var unit = KnownUnits.Find(World.EventUnit());
            if (unit == null)
            {
                return;
            }

            var inOrder = new List<Area>(areas);
            for (var i = 0; i < inOrder.Count; i++)
            {
                inOrder[i].Revive(unit);
            }
        }

        /// <summary>A unit, item or destructable is ending, while its place can still be read: a unit leaves every area.</summary>
        private void Ending(Widget widget)
        {
            var unit = widget as Unit;
            if (unit != null)
            {
                Dropped(unit);
            }
        }

        /// <summary>Takes a unit out of every area, in the order they were added.</summary>
        private void Dropped(Unit unit)
        {
            var inOrder = new List<Area>(areas);
            for (var i = 0; i < inOrder.Count; i++)
            {
                inOrder[i].Drop(unit);
            }
        }
    }
}
