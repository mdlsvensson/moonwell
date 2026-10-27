using System;

namespace Moonwell.Testing
{
    /// <summary>
    /// A case from the full audit's finding MT3: a small polygon far from (0, 0) has its area and center right in floats.
    /// .NET's float is single precision like the game's, so .NET sees the rounding; Lua on the test host has 64-bit numbers,
    /// where it passes either way.
    /// </summary>
    public static class FarPolygonCases
    {
        /// <summary>
        /// A triangle 40 across at (15000, 15000), whose center was 43 off and outside it, and other small shapes as far out,
        /// each against the same sums worked out in doubles.
        /// </summary>
        public static void AreaAndCenter(FakeWorld world)
        {
            Check(new Polygon(new Vector2(15000f, 15000f), new Vector2(15040f, 15003f), new Vector2(15009f, 15042f)), "a triangle at (15000, 15000)");
            Check(new Polygon(new Vector2(-29000.5f, 27000.25f), new Vector2(-28990.5f, 27000.25f), new Vector2(-28990.5f, 27010.25f), new Vector2(-29000.5f, 27010.25f)), "a square of 10 at (-29000, 27000)");
            Check(new Polygon(
                new Vector2(20011f, -18003f), new Vector2(20041f, -18003f), new Vector2(20041f, -17991f),
                new Vector2(20023f, -17991f), new Vector2(20023f, -17973f), new Vector2(20011f, -17973f)), "an L 30 across at (20000, -18000)");
            Check(new Polygon(
                new Vector2(-24984.3f, -26007.1f), new Vector2(-24997.9f, -25991.3f), new Vector2(-25013.7f, -25995.9f),
                new Vector2(-25012.1f, -26014.8f), new Vector2(-24995.6f, -26019.4f)), "a pentagon 30 across at (-25000, -26000)");
        }

        /// <summary>Fails unless the polygon's area is within 0.01% and its center within 0.01 of the sums in doubles.</summary>
        private static void Check(Polygon polygon, string what)
        {
            var points = polygon.Points;
            var first = points[0];
            var twice = 0.0;
            var cx = 0.0;
            var cy = 0.0;
            for (var i = 0; i < points.Count; i++)
            {
                var a = points[i];
                var b = points[(i + 1) % points.Count];
                var ax = (double)a.X - first.X;
                var ay = (double)a.Y - first.Y;
                var bx = (double)b.X - first.X;
                var by = (double)b.Y - first.Y;
                var cross = ax * by - ay * bx;
                twice += cross;
                cx += (ax + bx) * cross;
                cy += (ay + by) * cross;
            }

            var area = Math.Abs(twice) / 2.0;
            var centerX = first.X + cx / (3.0 * twice);
            var centerY = first.Y + cy / (3.0 * twice);
            Expect.True(Math.Abs(polygon.Area - area) <= area * 0.0001, what + ": area " + polygon.Area + ", in doubles " + area);
            var off = Math.Sqrt((polygon.Center.X - centerX) * (polygon.Center.X - centerX) + (polygon.Center.Y - centerY) * (polygon.Center.Y - centerY));
            Expect.True(off < 0.01, what + ": center " + polygon.Center + ", " + off + " from the one in doubles");
            Expect.True(polygon.Contains(polygon.Center), what + ": the center is inside");
        }
    }
}
