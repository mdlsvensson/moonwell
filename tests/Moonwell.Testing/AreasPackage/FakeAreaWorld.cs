using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game the areas package's cases run against: regions that remember the rects and cells they were given, enter, leave, death and revive events a case sends by hand, units at points, and units without a place.</summary>
    public sealed class FakeAreaWorld : IAreaWorld
    {
        /// <summary>What a fake region remembers.</summary>
        public sealed class Made
        {
            /// <summary>The region handle the package holds.</summary>
            public region Handle = new region();

            /// <summary>The rects added, four floats each (left, bottom, right, top).</summary>
            public List<float> Rects = new List<float>();

            /// <summary>The cells added, two floats each (x, y).</summary>
            public List<float> Cells = new List<float>();

            /// <summary>Whether the region has been destroyed, with its triggers.</summary>
            public bool Destroyed;

            /// <summary>Whether Listen was called for it.</summary>
            public bool Listening;

            /// <summary>What the package runs when a unit enters.</summary>
            public Action? Entered;

            /// <summary>What the package runs when a unit leaves.</summary>
            public Action? Left;

            /// <summary>The units a case has put inside it.</summary>
            public List<unit> Inside = new List<unit>();
        }

        /// <summary>A unit at a point.</summary>
        private sealed class Standing
        {
            internal unit Unit = new unit();

            internal float X;

            internal float Y;
        }

        /// <summary>Every region the package made, in order.</summary>
        public List<Made> Regions = new List<Made>();

        /// <summary>The units UnitsInRect gives, with their points.</summary>
        private readonly List<Standing> standing = new List<Standing>();

        /// <summary>The units HasPlace answers false for.</summary>
        public List<unit> NoPlace = new List<unit>();

        /// <summary>The unit of the event being delivered.</summary>
        public unit? Event;

        /// <summary>How many times ListenDeaths was called.</summary>
        public int DeathListens;

        /// <summary>What the package runs when a unit dies.</summary>
        public Action? Died;

        /// <summary>What the package runs when a hero is revived.</summary>
        public Action? Revived;

        /// <summary>Puts a unit at a point, for UnitsInRect.</summary>
        public void Stand(unit target, float x, float y)
        {
            var one = new Standing();
            one.Unit = target;
            one.X = x;
            one.Y = y;
            standing.Add(one);
        }

        /// <summary>Sends an enter event for a unit into a region, and puts the unit inside it.</summary>
        public void Enter(Made region, unit target)
        {
            region.Inside.Add(target);
            Send(region.Entered, target);
        }

        /// <summary>Sends a leave event for a unit out of a region, and takes the unit out of it.</summary>
        public void Leave(Made region, unit target)
        {
            region.Inside.Remove(target);
            Send(region.Left, target);
        }

        /// <summary>Sends a death event for a unit.</summary>
        public void Die(unit target)
        {
            Send(Died, target);
        }

        /// <summary>Sends a hero revive event for a unit.</summary>
        public void Revive(unit target)
        {
            Send(Revived, target);
        }

        /// <summary>Runs an event's action with the unit set, and puts the previous one back.</summary>
        private void Send(Action? action, unit target)
        {
            var before = Event;
            Event = target;
            try
            {
                if (action != null)
                {
                    action();
                }
            }
            finally
            {
                Event = before;
            }
        }

        /// <summary>The record of a region handle.</summary>
        public Made Of(region handle)
        {
            for (var i = 0; i < Regions.Count; i++)
            {
                if (Regions[i].Handle == handle)
                {
                    return Regions[i];
                }
            }

            throw new ExpectationFailed("The package used a region the fake did not make.");
        }

        /// <summary>Makes an empty region.</summary>
        public region NewRegion()
        {
            var made = new Made();
            Regions.Add(made);
            return made.Handle;
        }

        /// <summary>Remembers a rect.</summary>
        public void AddRect(region r, float minX, float minY, float maxX, float maxY)
        {
            var list = Of(r).Rects;
            list.Add(minX);
            list.Add(minY);
            list.Add(maxX);
            list.Add(maxY);
        }

        /// <summary>Remembers a cell.</summary>
        public void AddCell(region r, float x, float y)
        {
            var list = Of(r).Cells;
            list.Add(x);
            list.Add(y);
        }

        /// <summary>Remembers what to run on enter and on leave.</summary>
        public void Listen(region r, Action entered, Action left)
        {
            var made = Of(r);
            made.Listening = true;
            made.Entered = entered;
            made.Left = left;
        }

        /// <summary>The unit of the event being delivered.</summary>
        public unit EventUnit() => Event!;

        /// <summary>Marks a region destroyed.</summary>
        public void DestroyRegion(region r)
        {
            var made = Of(r);
            made.Destroyed = true;
            made.Entered = null;
            made.Left = null;
        }

        /// <summary>Whether a case has put the unit inside the region.</summary>
        public bool IsInside(region r, unit u) => Of(r).Inside.Contains(u);

        /// <summary>Adds the units standing in the rect, in the order they were put down.</summary>
        public void UnitsInRect(float minX, float minY, float maxX, float maxY, List<unit> into)
        {
            for (var i = 0; i < standing.Count; i++)
            {
                var one = standing[i];
                if (one.X >= minX && one.X <= maxX && one.Y >= minY && one.Y <= maxY)
                {
                    into.Add(one.Unit);
                }
            }
        }

        /// <summary>Remembers what to run on a death and on a revive.</summary>
        public void ListenDeaths(Action died, Action revived)
        {
            DeathListens++;
            Died = died;
            Revived = revived;
        }

        /// <summary>Whether the unit has a place: it is not one a case took the place from.</summary>
        public bool HasPlace(unit u) => !NoPlace.Contains(u);
    }
}
