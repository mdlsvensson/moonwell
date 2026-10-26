using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>
    /// The package's one tracker: the areas in the order they were added, the game's death and revive events and core's
    /// ending hook (set up when the first area is added), and the one timer that runs every area's OnUnitStays and place
    /// check. The timer steps every 1/32 second while some area has a unit inside or away, and ends when none has.
    /// </summary>
    internal sealed class AreaTracker
    {
        /// <summary>The seconds of a step of the package timer.</summary>
        internal const float Interval = 0.03125f;

        /// <summary>How many steps are a place check: every 8th, 0.25 second.</summary>
        private const int PlaceCheckSteps = 8;

        /// <summary>The tracker of the world and game now; made at the first area added after an install or a runtime reset.</summary>
        private static AreaTracker? current;

        /// <summary>The world this tracker talks to.</summary>
        internal readonly IAreaWorld World;

        /// <summary>The game this tracker was made for; a runtime reset makes another, and the tracker is stale.</summary>
        private readonly Game game;

        /// <summary>The areas in the order they were added.</summary>
        private readonly List<Area> areas = new List<Area>();

        /// <summary>The package timer; null while no area has a unit.</summary>
        private Timer? timer;

        /// <summary>How many steps the running timer has made.</summary>
        internal int Steps;

        /// <summary>How many areas have a unit inside or away.</summary>
        private int activeAreas;

        /// <summary>Makes a tracker for a world and a game.</summary>
        private AreaTracker(IAreaWorld world, Game game)
        {
            World = world;
            this.game = game;
        }

        /// <summary>Whether the package timer is running now; for the cases.</summary>
        internal static bool Running
        {
            get { return current != null && current.timer != null; }
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
            }

            return current;
        }

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

        /// <summary>An area starts or stops counting for the package timer: the first starts it, the last ends it.</summary>
        internal void Activity(bool started)
        {
            if (started)
            {
                activeAreas++;
                if (activeAreas == 1)
                {
                    Steps = 0;
                    timer = Timer.Every(Interval, Step, game);
                }
            }
            else
            {
                activeAreas--;
                if (activeAreas == 0 && timer != null)
                {
                    var ending = timer;
                    timer = null;
                    ending.End();
                }
            }
        }

        /// <summary>Adds the rects and the cells of a shape to a region: a rect or region part is a rect, a polygon or a circle part its 32 cells; a union adds those of its parts.</summary>
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
                World.AddRect(made, bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y);
                return;
            }

            var centres = new List<float>();
            AreaCells.Of(shape, centres);
            for (var i = 0; i < centres.Count; i += 2)
            {
                World.AddCell(made, centres[i], centres[i + 1]);
            }
        }

        /// <summary>One step of the package timer: every 8th step each area's place check, then each area whose Tick is due runs OnUnitStays; in the order the areas were added. A step stops when the timer ends or restarts under it.</summary>
        private void Step()
        {
            var running = timer;
            Steps++;
            var step = Steps;
            var inOrder = new List<Area>(areas);
            if (step % PlaceCheckSteps == 0)
            {
                for (var i = 0; i < inOrder.Count; i++)
                {
                    if (timer != running)
                    {
                        return;
                    }

                    inOrder[i].CheckPlaces();
                }
            }

            for (var i = 0; i < inOrder.Count; i++)
            {
                if (timer != running)
                {
                    return;
                }

                inOrder[i].Stay(step);
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
