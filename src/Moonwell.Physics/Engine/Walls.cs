using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>
    /// The map's walls, in the order they were made. A wall is a gizmo, so it has a place in the interactions grid, but the
    /// grid keys a thing by its middle, and a missile or a pushed unit touching the end of a long wall may be far from its
    /// middle. So the package keeps its own list of walls, and each check runs down it, passing over a wall whose box (its
    /// two ends, widened by the ball's radius) does not hold the move: a few float compares a wall. Maps have few walls.
    /// </summary>
    internal static class Walls
    {
        /// <summary>The walls, in the order they were made; an ended one stays until the list is packed. Made on first use after an install. It holds no null, so the checks read it raw.</summary>
        private static List<Wall>? walls;

        /// <summary>Whether a wall has ended since the list was last packed.</summary>
        private static bool dropped;

        /// <summary>How many checks are running down the list, so it is not packed under one.</summary>
        private static int walking;

        /// <summary>Forgets every wall; an install starts afresh.</summary>
        internal static void Reset()
        {
            walls = null;
            dropped = false;
            walking = 0;
        }

        /// <summary>Adds a wall at the end of the list, as it is made.</summary>
        internal static void Add(Wall wall)
        {
            if (walls == null)
            {
                walls = new List<Wall>();
            }

            Raw.Append(walls, wall);
        }

        /// <summary>Notes that a wall has ended, so the list is packed before the next check runs down it.</summary>
        internal static void Dropped()
        {
            dropped = true;
        }

        /// <summary>Starts a check's walk down the walls: the list, packed first when a wall has ended and no walk is running; null when there is no wall. A walk begun must be ended with Done.</summary>
        internal static List<Wall>? Walk()
        {
            var list = walls;
            if (list == null)
            {
                return null;
            }

            if (dropped && walking == 0)
            {
                Pack(list);
            }

            if (list.Count == 0)
            {
                return null;
            }

            walking++;
            return list;
        }

        /// <summary>Ends a walk Walk began.</summary>
        internal static void Done()
        {
            walking--;
        }

        /// <summary>
        /// Whether a wall stands across the line a pushed unit looks along, from its place to the point ahead of it: the two
        /// segments cross (touching counts), and the wall is endless or taller than the unit is lifted (a flyer: its fly
        /// height). Worked out in floats, so nothing is made.
        /// </summary>
        internal static bool Blocks(Pushed push, float fromX, float fromY, float toX, float toY)
        {
            var list = Walk();
            if (list == null)
            {
                return false;
            }

            var rx = toX - fromX;
            var ry = toY - fromY;
            var lowX = fromX < toX ? fromX : toX;
            var highX = fromX < toX ? toX : fromX;
            var lowY = fromY < toY ? fromY : toY;
            var highY = fromY < toY ? toY : fromY;
            var blocked = false;
            var lift = -1.0f;
            var count = list.Count;
            for (var i = 0; i < count && !blocked; i++)
            {
                var wall = Raw.At(list, i);
                if (wall.ending)
                {
                    continue;
                }

                var entry = wall.entry!;
                var shiftX = entry.X - wall.madeX;
                var shiftY = entry.Y - wall.madeY;
                var x1 = wall.x1 + shiftX;
                var y1 = wall.y1 + shiftY;
                var x2 = wall.x2 + shiftX;
                var y2 = wall.y2 + shiftY;
                if ((x1 < lowX && x2 < lowX) || (x1 > highX && x2 > highX) || (y1 < lowY && y2 < lowY) || (y1 > highY && y2 > highY))
                {
                    continue;
                }

                var sx = x2 - x1;
                var sy = y2 - y1;
                var across = rx * sy - ry * sx;
                if (across == 0.0f)
                {
                    // Along the wall's line, or no move: it does not cross it.
                    continue;
                }

                var qx = x1 - fromX;
                var qy = y1 - fromY;
                var t = (qx * sy - qy * sx) / across;
                var u = (qx * ry - qy * rx) / across;
                if (t < 0.0f || t > 1.0f || u < 0.0f || u > 1.0f)
                {
                    continue;
                }

                if (wall.height > 0.0f)
                {
                    if (lift < 0.0f)
                    {
                        // Read only when a wall that is not endless is crossed: a flyer's height is a read of the game.
                        lift = push.lifted ? push.height : (push.flyer ? push.unit.FlyHeight : 0.0f);
                    }

                    if (lift >= wall.height)
                    {
                        continue;
                    }
                }

                blocked = true;
            }

            Done();
            return blocked;
        }

        /// <summary>Takes the ended walls out of the list, keeping the order of the rest.</summary>
        private static void Pack(List<Wall> list)
        {
            dropped = false;
            var kept = 0;
            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var wall = Raw.At(list, i);
                if (!wall.ending)
                {
                    Raw.Put(list, kept, wall);
                    kept++;
                }
            }

            Raw.Truncate(list, kept);
        }
    }
}
