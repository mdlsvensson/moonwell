namespace Moonwell.Testing
{
    /// <summary>Cases for the maths package's vector methods on units and items, on the fake world.</summary>
    public static class ActorMathsCases
    {
        private static Unit UnitAt(FakeWorld world, float x, float y)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            unit.PlaceAt(x, y);
            return unit;
        }

        private static Item ItemAt(FakeWorld world, float x, float y)
        {
            var item = Item.Of(world.NewItem(ObjectTypes.FourCC("ratc")))!;
            item.MoveTo(x, y);
            return item;
        }

        /// <summary>Position reads the unit's and the item's place.</summary>
        public static void Position(FakeWorld world)
        {
            Expect.True(UnitAt(world, 100.5f, -50f).Position() == new Vector2(100.5f, -50f), "unit");
            Expect.True(ItemAt(world, 7f, 8f).Position() == new Vector2(7f, 8f), "item");
        }

        /// <summary>PlaceAt and MoveTo reach the same world calls as core's float methods of those names.</summary>
        public static void PlaceAndMove(FakeWorld world)
        {
            var unit = UnitAt(world, 0f, 0f);
            world.UnitCalls.Clear();
            unit.PlaceAt(new Vector2(10f, 20f));
            Expect.Equal("place", world.UnitCalls[world.UnitCalls.Count - 1], "PlaceAt places");
            Expect.True(unit.Position() == new Vector2(10f, 20f), "placed at (10, 20)");
            unit.MoveTo(new Vector2(30f, 40f));
            Expect.Equal("move", world.UnitCalls[world.UnitCalls.Count - 1], "MoveTo moves");
            Expect.True(unit.Position() == new Vector2(30f, 40f), "moved to (30, 40)");
            var item = ItemAt(world, 0f, 0f);
            item.MoveTo(new Vector2(5f, 6f));
            Expect.True(item.Position() == new Vector2(5f, 6f), "item moved to (5, 6)");
        }

        /// <summary>Distance and facing between units, items and points.</summary>
        public static void DistanceAndAngle(FakeWorld world)
        {
            var a = UnitAt(world, 0f, 0f);
            var b = UnitAt(world, 0f, 300f);
            var item = ItemAt(world, -10f, 0f);
            Expect.Near(300f, a.DistanceTo(b), "unit to unit");
            Expect.Near(90f, a.AngleTo(b), "unit to unit angle");
            Expect.Near(10f, a.DistanceTo(item), "unit to item");
            Expect.Near(180f, a.AngleTo(item), "unit to item angle");
            Expect.Near(5f, a.DistanceTo(new Vector2(3f, 4f)), "unit to point");
            Expect.Near(270f, a.AngleTo(new Vector2(0f, -1f)), "unit to point angle");
            Expect.Near(300.1666f, item.DistanceTo(b), "item to unit");
            Expect.Near(10f, item.DistanceTo(a), "item to unit, the other way");
            Expect.Near(0f, item.AngleTo(a), "item to unit angle");
            Expect.Near(10f, item.DistanceTo(new Vector2(0f, 0f)), "item to point");
            Expect.Near(10f, item.DistanceTo(ItemAt(world, 0f, 0f)), "item to item");
        }

        /// <summary>A destructable has a position, and the distance and facing between it and units, items, destructables and points work both ways.</summary>
        public static void DestructableDistances(FakeWorld world)
        {
            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlaceDestructable(handle, 30f, 40f);
            var tree = Destructable.Of(handle)!;
            var other = Destructable.Of(world.NewDestructable(ObjectTypes.FourCC("dfoo")))!;
            Expect.True(tree.Position() == new Vector2(30f, 40f), "position");
            var unit = UnitAt(world, 30f, 140f);
            var item = ItemAt(world, 20f, 40f);
            Expect.Near(100f, tree.DistanceTo(unit), "destructable to unit");
            Expect.Near(90f, tree.AngleTo(unit), "destructable to unit angle");
            Expect.Near(100f, unit.DistanceTo(tree), "unit to destructable");
            Expect.Near(270f, unit.AngleTo(tree), "unit to destructable angle");
            Expect.Near(10f, tree.DistanceTo(item), "destructable to item");
            Expect.Near(180f, tree.AngleTo(item), "destructable to item angle");
            Expect.Near(10f, item.DistanceTo(tree), "item to destructable");
            Expect.Near(0f, item.AngleTo(tree), "item to destructable angle");
            Expect.Near(5f, tree.DistanceTo(new Vector2(33f, 44f)), "destructable to point");
            Expect.Near(270f, tree.AngleTo(new Vector2(30f, 0f)), "destructable to point angle");
            Expect.Near(Maths.Distance(30f, 40f, 100f, -50f), tree.DistanceTo(other), "destructable to destructable");
            Expect.Near(Maths.AngleBetween(30f, 40f, 100f, -50f), tree.AngleTo(other), "destructable to destructable angle");
            Expect.Near(Maths.Distance(100f, -50f, 30f, 40f), other.DistanceTo(tree), "destructable to destructable, the other way");
            Expect.Near(Maths.AngleBetween(100f, -50f, 30f, 40f), other.AngleTo(tree), "destructable to destructable angle, the other way");
        }

        /// <summary>FaceToward turns as setting Facing does; FaceNow turns at once.</summary>
        public static void Face(FakeWorld world)
        {
            var unit = UnitAt(world, 0f, 0f);
            unit.FaceToward(new Vector2(0f, -5f));
            Expect.Equal("turn", world.UnitCalls[world.UnitCalls.Count - 1], "FaceToward turns");
            Expect.Near(270f, world.TurnTargets[unit.Handle], "toward 270");
            unit.FaceNow(new Vector2(-5f, 0f));
            Expect.Equal("face-now", world.UnitCalls[world.UnitCalls.Count - 1], "FaceNow faces at once");
            Expect.Near(180f, unit.Facing, "now 180");
        }
    }
}
