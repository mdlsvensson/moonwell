namespace Moonwell.Testing
{
    /// <summary>Cases from the full audit's raycast finding F5: a segment through a polygon's corner goes in with a hit.</summary>
    public static class CornerRayCases
    {
        /// <summary>
        /// Segments that passed in through a corner with no hit, found among 200,000 aimed through corners of random
        /// pentagons: the first four by the code before the audit's fixes, the last five by the walls without their ends'
        /// skin. Each now hits at the corner.
        /// </summary>
        public static void Found(FakeWorld world)
        {
            var a = new Polygon(new Vector2(-366.52216f, 4426.0254f), new Vector2(-526.03467f, 4557.3613f), new Vector2(-768.4441f, 4546.7153f), new Vector2(-780.4783f, 4241.618f), new Vector2(-522.855f, 4164.793f));
            AtCorner(a, 0, new Vector2(-357.20502f, 4422.9194f), new Vector2(-372.49533f, 4428.0166f), "a");
            var b = new Polygon(new Vector2(3394.3835f, 2616.1816f), new Vector2(3200.4448f, 2881.1318f), new Vector2(2995.337f, 2833.1653f), new Vector2(3024.8733f, 2515.7378f), new Vector2(3276.0544f, 2472.6655f));
            AtCorner(b, 2, new Vector2(2979.743f, 2893.4622f), new Vector2(3004.1829f, 2798.961f), "b");
            var c = new Polygon(new Vector2(1138.6523f, -1606.2911f), new Vector2(1083.2966f, -1526.1309f), new Vector2(983.60046f, -1573.7225f), new Vector2(989.47687f, -1649.3743f), new Vector2(1088.1171f, -1704.3693f));
            AtCorner(c, 4, new Vector2(1130.6285f, -1739.4847f), new Vector2(1052.8396f, -1675.2292f), "c");
            var d = new Polygon(new Vector2(-956.98535f, -533.6432f), new Vector2(-1115.285f, -370.21637f), new Vector2(-1346.746f, -437.3181f), new Vector2(-1389.6362f, -771.457f), new Vector2(-1084.6042f, -790.74524f));
            AtCorner(d, 0, new Vector2(-867.769f, -526.1274f), new Vector2(-1006.4068f, -537.8066f), "d");
            var e = new Polygon(new Vector2(59.348186f, -101.76717f), new Vector2(48.052353f, -82.684265f), new Vector2(9.227209f, -94.65442f), new Vector2(-0.12073326f, -122.48476f), new Vector2(30.13446f, -137.38776f));
            AtCorner(e, 0, new Vector2(139.64934f, -124.185135f), new Vector2(24.787827f, -92.118835f), "e1");
            AtCorner(e, 0, new Vector2(140.94608f, -51.576485f), new Vector2(36.971325f, -115.531136f), "e2");
            AtCorner(e, 0, new Vector2(119.82054f, -168.26575f), new Vector2(29.350798f, -68.78047f), "e3");
            AtCorner(e, 1, new Vector2(101.68861f, -52.446815f), new Vector2(23.979097f, -96.25557f), "e4");
            AtCorner(e, 2, new Vector2(-55.006813f, -46.931206f), new Vector2(33.159332f, -112.435f), "e5");
        }

        /// <summary>Fails unless the segment hits the polygon within 0.05 of a corner.</summary>
        private static void AtCorner(Polygon polygon, int corner, Vector2 from, Vector2 to, string what)
        {
            Expect.True(Raycast.Polygon(from, to, polygon, out var hit), what + ": hits");
            var off = hit.Point.DistanceTo(polygon.Points[corner]);
            Expect.True(off < 0.05f, what + ": at the corner, was " + off + " from it");
        }

        /// <summary>
        /// A segment aimed through a corner of a pentagon far from (0, 0), from outside into it, hits it: 2,000 of them, 400
        /// through each corner from directions across its inside (audit F5: some used to pass in with no hit).
        /// </summary>
        public static void ThroughCorners(FakeWorld world)
        {
            var c = new Vector2(5003.37f, -6011.59f);
            var pentagon = BounceCases.Pentagon(c);
            var points = pentagon.Points;
            var misses = 0;
            var first = "";
            var farthest = 0f;
            for (var i = 0; i < 5; i++)
            {
                var corner = points[i];
                var before = points[(i + 4) % 5];
                var after = points[(i + 1) % 5];
                var toBefore = (before - corner).Angle;
                var toAfter = (after - corner).Angle;
                var across = Angles.Difference(toAfter, toBefore);
                for (var k = 0; k < 400; k++)
                {
                    // Directions strictly between the two sides at the corner, so the segment goes into the pentagon there.
                    var inward = Angles.Normalize(toAfter + across * (k + 0.5f) / 400f);
                    var direction = Vector2.FromAngle(inward, 1f);
                    var from = corner - direction * (20f + k * 0.37f);
                    var to = corner + direction * (15f + k * 0.11f);
                    if (!Raycast.Polygon(from, to, pentagon, out var hit))
                    {
                        misses++;
                        if (first == "")
                        {
                            first = "corner " + i + " at " + inward;
                        }
                    }
                    else if (hit.Point.DistanceTo(corner) > farthest)
                    {
                        farthest = hit.Point.DistanceTo(corner);
                    }
                }
            }

            Expect.Equal(0, misses, "segments through a corner that passed in with no hit (the first: " + first + ")");
            Expect.True(farthest < 0.05f, "the farthest hit from its corner was " + farthest);
        }
    }
}
