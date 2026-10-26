using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// Finds the things that have a place in a range, a rect, a polygon, along a line, and the closest. T is a class: Unit, Item,
    /// Destructable, Gizmo, a map's gizmo class, or Actor for all of them. A search keeps what is of the class T, has a place
    /// (a unit that is alive, shown and not loaded; an item on the ground; a destructable that is alive; a gizmo that has not
    /// ended), has its centre inside the shape, and is accepted by the filter. Each shape has a form that returns a new list, for
    /// a cast or a click, and a form that fills a list the map keeps (cleared first, returns the count), for code that runs
    /// often. Results come in the grid's order, the same on every machine; Closest gives the nearest first and keeps the grid's
    /// order among things at the same distance. Inside Local.For a search reads and changes nothing: it tracks no kind, and keeps no place it reads.
    /// </summary>
    public static class Find
    {
        /// <summary>The entries of the cells a search looks in. One list is shared: a search adds its own candidates at the end and takes them off again when it is done, so a Find from inside a filter or a reaction, while an outer search walks its candidates, never changes what the outer one walks. An interaction's check adds its own at the end too, and takes them off with Release as a search does.</summary>
        internal static readonly List<Entry> candidates = new List<Entry>();

        /// <summary>The squared distances of the results a Closest search has kept, in step with its results. Shared in the same way as the candidates.</summary>
        private static readonly List<float> distances = new List<float>();

        /// <summary>The things of class T with their centre within range of a point, as a new list.</summary>
        [LocalSafe]
        public static List<T> InRange<T>(float x, float y, float range, Func<T, bool>? where = null)
            where T : Actor
        {
            var results = new List<T>();
            InRange<T>(x, y, range, results, where);
            return results;
        }

        /// <summary>The things of class T with their centre within range of a point, as a new list.</summary>
        [LocalSafe]
        public static List<T> InRange<T>(Vector2 center, float range, Func<T, bool>? where = null)
            where T : Actor
        {
            var results = new List<T>();
            InRange<T>(center.X, center.Y, range, results, where);
            return results;
        }

        /// <summary>Clears the list and fills it with the things of class T with their centre within range of a point; returns how many.</summary>
        [LocalSafe]
        public static int InRange<T>(float x, float y, float range, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            results.Clear();
            Ask<T>();
            if (range < 0.0f)
            {
                return 0;
            }

            var limit = range * range;
            var start = Gather<T>(x - range, y - range, x + range, y + range);
            try
            {
                var end = candidates.Count;
                var px = 0.0f;
                var py = 0.0f;
                for (var i = start; i < end; i++)
                {
                    var entry = candidates[i];
                    var thing = Placed<T>(entry, out px, out py);
                    if (thing == null)
                    {
                        continue;
                    }

                    var dx = px - x;
                    var dy = py - y;
                    if (dx * dx + dy * dy > limit)
                    {
                        continue;
                    }

                    if (where != null && !where(thing))
                    {
                        continue;
                    }

                    results.Add(thing);
                }
            }
            finally
            {
                Release(start);
            }

            return results.Count;
        }

        /// <summary>Clears the list and fills it with the things of class T with their centre within range of a point; returns how many.</summary>
        [LocalSafe]
        public static int InRange<T>(Vector2 center, float range, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            return InRange<T>(center.X, center.Y, range, results, where);
        }

        /// <summary>The things of class T with their centre inside the bounds, the edges included, as a new list.</summary>
        [LocalSafe]
        public static List<T> InRect<T>(Bounds bounds, Func<T, bool>? where = null)
            where T : Actor
        {
            var results = new List<T>();
            InRect<T>(bounds, results, where);
            return results;
        }

        /// <summary>Clears the list and fills it with the things of class T with their centre inside the bounds, the edges included; returns how many.</summary>
        [LocalSafe]
        public static int InRect<T>(Bounds bounds, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            results.Clear();
            Ask<T>();
            var minX = bounds.Min.X;
            var minY = bounds.Min.Y;
            var maxX = bounds.Max.X;
            var maxY = bounds.Max.Y;
            var start = Gather<T>(minX, minY, maxX, maxY);
            try
            {
                var end = candidates.Count;
                var px = 0.0f;
                var py = 0.0f;
                for (var i = start; i < end; i++)
                {
                    var entry = candidates[i];
                    var thing = Placed<T>(entry, out px, out py);
                    if (thing == null)
                    {
                        continue;
                    }

                    if (px < minX || px > maxX || py < minY || py > maxY)
                    {
                        continue;
                    }

                    if (where != null && !where(thing))
                    {
                        continue;
                    }

                    results.Add(thing);
                }
            }
            finally
            {
                Release(start);
            }

            return results.Count;
        }

        /// <summary>The things of class T with their centre inside the polygon, a point on an edge included, as a new list.</summary>
        [LocalSafe]
        public static List<T> InPolygon<T>(Polygon polygon, Func<T, bool>? where = null)
            where T : Actor
        {
            var results = new List<T>();
            InPolygon<T>(polygon, results, where);
            return results;
        }

        /// <summary>Clears the list and fills it with the things of class T with their centre inside the polygon, a point on an edge included; returns how many.</summary>
        [LocalSafe]
        public static int InPolygon<T>(Polygon polygon, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            results.Clear();
            Ask<T>();
            var bounds = polygon.Bounds;
            var start = Gather<T>(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y);
            try
            {
                var end = candidates.Count;
                var px = 0.0f;
                var py = 0.0f;
                for (var i = start; i < end; i++)
                {
                    var entry = candidates[i];
                    var thing = Placed<T>(entry, out px, out py);
                    if (thing == null)
                    {
                        continue;
                    }

                    if (!polygon.Contains(px, py))
                    {
                        continue;
                    }

                    if (where != null && !where(thing))
                    {
                        continue;
                    }

                    results.Add(thing);
                }
            }
            finally
            {
                Release(start);
            }

            return results.Count;
        }

        /// <summary>The things of class T with their centre within half the width of the segment from one point to another, as a new list.</summary>
        [LocalSafe]
        public static List<T> AlongLine<T>(float x1, float y1, float x2, float y2, float width, Func<T, bool>? where = null)
            where T : Actor
        {
            var results = new List<T>();
            AlongLine<T>(x1, y1, x2, y2, width, results, where);
            return results;
        }

        /// <summary>The things of class T with their centre within half the width of the segment from one point to another, as a new list.</summary>
        [LocalSafe]
        public static List<T> AlongLine<T>(Vector2 from, Vector2 to, float width, Func<T, bool>? where = null)
            where T : Actor
        {
            var results = new List<T>();
            AlongLine<T>(from.X, from.Y, to.X, to.Y, width, results, where);
            return results;
        }

        /// <summary>Clears the list and fills it with the things of class T with their centre within half the width of the segment from one point to another; returns how many.</summary>
        [LocalSafe]
        public static int AlongLine<T>(float x1, float y1, float x2, float y2, float width, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            results.Clear();
            Ask<T>();
            if (width < 0.0f)
            {
                return 0;
            }

            var half = width / 2.0f;
            var limit = half * half;
            var dx = x2 - x1;
            var dy = y2 - y1;
            var lengthSquared = dx * dx + dy * dy;
            var start = Gather<T>(
                (x1 < x2 ? x1 : x2) - half,
                (y1 < y2 ? y1 : y2) - half,
                (x1 > x2 ? x1 : x2) + half,
                (y1 > y2 ? y1 : y2) + half);
            try
            {
                var end = candidates.Count;
                var px = 0.0f;
                var py = 0.0f;
                for (var i = start; i < end; i++)
                {
                    var entry = candidates[i];
                    var thing = Placed<T>(entry, out px, out py);
                    if (thing == null)
                    {
                        continue;
                    }

                    var along = 0.0f;
                    if (lengthSquared > 0.0f)
                    {
                        along = ((px - x1) * dx + (py - y1) * dy) / lengthSquared;
                        if (along < 0.0f)
                        {
                            along = 0.0f;
                        }
                        else if (along > 1.0f)
                        {
                            along = 1.0f;
                        }
                    }

                    var cx = x1 + dx * along - px;
                    var cy = y1 + dy * along - py;
                    if (cx * cx + cy * cy > limit)
                    {
                        continue;
                    }

                    if (where != null && !where(thing))
                    {
                        continue;
                    }

                    results.Add(thing);
                }
            }
            finally
            {
                Release(start);
            }

            return results.Count;
        }

        /// <summary>Clears the list and fills it with the things of class T with their centre within half the width of the segment from one point to another; returns how many.</summary>
        [LocalSafe]
        public static int AlongLine<T>(Vector2 from, Vector2 to, float width, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            return AlongLine<T>(from.X, from.Y, to.X, to.Y, width, results, where);
        }

        /// <summary>The nearest thing of class T within maxRange of a point that the filter accepts; null when there is none. Of two at the same distance, the one the grid reads first. The filter is asked only of a candidate that could be kept.</summary>
        [LocalSafe]
        public static T? Closest<T>(float x, float y, float maxRange, Func<T, bool>? where = null)
            where T : Actor
        {
            Ask<T>();
            T? best = null;
            if (maxRange < 0.0f)
            {
                return best;
            }

            var limit = maxRange * maxRange;
            var bestSquared = limit;
            var start = Gather<T>(x - maxRange, y - maxRange, x + maxRange, y + maxRange);
            try
            {
                var end = candidates.Count;
                var px = 0.0f;
                var py = 0.0f;
                for (var i = start; i < end; i++)
                {
                    var entry = candidates[i];
                    var thing = Placed<T>(entry, out px, out py);
                    if (thing == null)
                    {
                        continue;
                    }

                    var dx = px - x;
                    var dy = py - y;
                    var squared = dx * dx + dy * dy;
                    if (squared > limit || (best != null && squared >= bestSquared))
                    {
                        continue;
                    }

                    if (where != null && !where(thing))
                    {
                        continue;
                    }

                    best = thing;
                    bestSquared = squared;
                }
            }
            finally
            {
                Release(start);
            }

            return best;
        }

        /// <summary>The nearest thing of class T within maxRange of a point that the filter accepts; null when there is none. Of two at the same distance, the one the grid reads first. The filter is asked only of a candidate that could be kept.</summary>
        [LocalSafe]
        public static T? Closest<T>(Vector2 center, float maxRange, Func<T, bool>? where = null)
            where T : Actor
        {
            return Closest<T>(center.X, center.Y, maxRange, where);
        }

        /// <summary>Clears the list and fills it with the nearest count things of class T within maxRange of a point, nearest first, and returns how many; things at the same distance come in the grid's order. The filter is asked only of a candidate that could be kept.</summary>
        [LocalSafe]
        public static int Closest<T>(float x, float y, int count, float maxRange, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            results.Clear();
            Ask<T>();
            if (count <= 0 || maxRange < 0.0f)
            {
                return 0;
            }

            var limit = maxRange * maxRange;
            var first = distances.Count;
            var start = Gather<T>(x - maxRange, y - maxRange, x + maxRange, y + maxRange);
            try
            {
                var end = candidates.Count;
                var px = 0.0f;
                var py = 0.0f;
                for (var i = start; i < end; i++)
                {
                    var entry = candidates[i];
                    var thing = Placed<T>(entry, out px, out py);
                    if (thing == null)
                    {
                        continue;
                    }

                    var dx = px - x;
                    var dy = py - y;
                    var squared = dx * dx + dy * dy;
                    if (squared > limit)
                    {
                        continue;
                    }

                    var kept = results.Count;
                    if (kept >= count && squared >= distances[first + kept - 1])
                    {
                        continue;
                    }

                    if (where != null && !where(thing))
                    {
                        continue;
                    }

                    if (kept >= count)
                    {
                        results.RemoveAt(kept - 1);
                        distances.RemoveAt(first + kept - 1);
                        kept--;
                    }

                    var at = kept;
                    while (at > 0 && distances[first + at - 1] > squared)
                    {
                        at--;
                    }

                    results.Insert(at, thing);
                    distances.Insert(first + at, squared);
                }
            }
            finally
            {
                while (distances.Count > first)
                {
                    distances.RemoveAt(distances.Count - 1);
                }

                Release(start);
            }

            return results.Count;
        }

        /// <summary>Clears the list and fills it with the nearest count things of class T within maxRange of a point, nearest first, and returns how many; things at the same distance come in the grid's order. The filter is asked only of a candidate that could be kept.</summary>
        [LocalSafe]
        public static int Closest<T>(Vector2 center, int count, float maxRange, List<T> results, Func<T, bool>? where = null)
            where T : Actor
        {
            return Closest<T>(center.X, center.Y, count, maxRange, results, where);
        }

        /// <summary>What a search is looking at now: how many candidates and squared distances the searches that are running hold. It is 0 whenever no search is running, however the searches ended.</summary>
        internal static int Pending
        {
            get { return candidates.Count + distances.Count; }
        }

        /// <summary>Asks for the kinds of thing T covers, which tracks the kinds not tracked yet, and fails in a development build for a class with no place. Inside Local.For it only checks the class: a search there may track nothing.</summary>
        private static void Ask<T>()
            where T : Actor
        {
            if (Local.Depth > 0)
            {
                Tracking.CheckPlace(typeof(T));
            }
            else
            {
                Tracking.NeedFor(typeof(T));
            }
        }

        /// <summary>Adds the entries of the cells a box touches, grown by the slack, to the end of the shared candidates; returns where they start. Inside Local.For it makes and installs nothing: it takes the cells of what is tracked, if there is a grid, and an entry of the search's own for each live thing of a kind nobody has asked for.</summary>
        private static int Gather<T>(float minX, float minY, float maxX, float maxY)
            where T : Actor
        {
            var start = candidates.Count;
            if (Local.Depth > 0)
            {
                var made = Interactions.GridIfMade;
                if (made != null)
                {
                    var slack = Interactions.Slack;
                    made.Collect(minX - slack, minY - slack, maxX + slack, maxY + slack, candidates);
                }

                Tracking.AddUntracked<T>(candidates);
                return start;
            }

            var spread = Interactions.Slack;
            Interactions.Grid.Collect(minX - spread, minY - spread, maxX + spread, maxY + spread, candidates);
            return start;
        }

        /// <summary>Takes the candidates a search or a check added off the end of the shared list, leaving those of the searches around it.</summary>
        internal static void Release(int start)
        {
            Raw.Truncate(candidates, start);
        }

        /// <summary>The thing of an entry when it is of class T and has a place now, with where it is through out; null otherwise. An entry of another class is not read. A place is read at most once a step and kept; inside Local.For it is read into locals and kept nowhere.</summary>
        private static T? Placed<T>(Entry entry, out float x, out float y)
            where T : Actor
        {
            x = 0.0f;
            y = 0.0f;
            var thing = entry.Thing as T;
            if (thing == null)
            {
                return null;
            }

            if (Local.Depth > 0)
            {
                return Tracking.Peek(entry, out x, out y) ? thing : null;
            }

            Tracking.ReadNow(entry);
            x = entry.X;
            y = entry.Y;
            return entry.HasPlace ? thing : null;
        }
    }
}
