namespace Moonwell
{
    /// <summary>
    /// Vector methods on units, items and destructables, from the maths package. Position is a method, not a property, as the package can
    /// add only methods. Distances and angles are worked out from the floats, with no vector made. Angles are in degrees.
    /// </summary>
    public static class ActorMaths
    {
        /// <summary>The unit's place (its X and Y).</summary>
        [LocalSafe]
        public static Vector2 Position(this Unit unit)
        {
            return new Vector2(unit.X, unit.Y);
        }

        /// <summary>Moves the unit through the game's pathing, as Unit.MoveTo(x, y): the game finds a free spot, and the unit's order ends.</summary>
        public static void MoveTo(this Unit unit, Vector2 place)
        {
            unit.MoveTo(place.X, place.Y);
        }

        /// <summary>Puts the unit at a place with no pathing and no bounds, as Unit.PlaceAt(x, y): its order goes on, which a push wants. Keep it inside the map yourself.</summary>
        public static void PlaceAt(this Unit unit, Vector2 place)
        {
            unit.PlaceAt(place.X, place.Y);
        }

        /// <summary>The distance from the unit to another.</summary>
        [LocalSafe]
        public static float DistanceTo(this Unit unit, Unit other)
        {
            return Maths.Distance(unit.X, unit.Y, other.X, other.Y);
        }

        /// <summary>The distance from the unit to an item.</summary>
        [LocalSafe]
        public static float DistanceTo(this Unit unit, Item item)
        {
            return Maths.Distance(unit.X, unit.Y, item.X, item.Y);
        }

        /// <summary>The distance from the unit to a destructable.</summary>
        [LocalSafe]
        public static float DistanceTo(this Unit unit, Destructable destructable)
        {
            return Maths.Distance(unit.X, unit.Y, destructable.X, destructable.Y);
        }

        /// <summary>The distance from the unit to a point.</summary>
        [LocalSafe]
        public static float DistanceTo(this Unit unit, Vector2 point)
        {
            return Maths.Distance(unit.X, unit.Y, point.X, point.Y);
        }

        /// <summary>The facing in degrees from the unit to another, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Unit unit, Unit other)
        {
            return Maths.AngleBetween(unit.X, unit.Y, other.X, other.Y);
        }

        /// <summary>The facing in degrees from the unit to an item, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Unit unit, Item item)
        {
            return Maths.AngleBetween(unit.X, unit.Y, item.X, item.Y);
        }

        /// <summary>The facing in degrees from the unit to a destructable, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Unit unit, Destructable destructable)
        {
            return Maths.AngleBetween(unit.X, unit.Y, destructable.X, destructable.Y);
        }

        /// <summary>The facing in degrees from the unit to a point, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Unit unit, Vector2 point)
        {
            return Maths.AngleBetween(unit.X, unit.Y, point.X, point.Y);
        }

        /// <summary>Turns the unit toward a point at its turn rate, as setting Facing does; a read just after still gives the old facing.</summary>
        public static void FaceToward(this Unit unit, Vector2 point)
        {
            unit.Facing = Maths.AngleBetween(unit.X, unit.Y, point.X, point.Y);
        }

        /// <summary>Turns the unit toward a point at once, as Unit.FaceNow(degrees).</summary>
        public static void FaceNow(this Unit unit, Vector2 point)
        {
            unit.FaceNow(Maths.AngleBetween(unit.X, unit.Y, point.X, point.Y));
        }

        /// <summary>The item's place (its X and Y).</summary>
        [LocalSafe]
        public static Vector2 Position(this Item item)
        {
            return new Vector2(item.X, item.Y);
        }

        /// <summary>Moves the item to a place, as Item.MoveTo(x, y).</summary>
        public static void MoveTo(this Item item, Vector2 place)
        {
            item.MoveTo(place.X, place.Y);
        }

        /// <summary>The distance from the item to a unit.</summary>
        [LocalSafe]
        public static float DistanceTo(this Item item, Unit unit)
        {
            return Maths.Distance(item.X, item.Y, unit.X, unit.Y);
        }

        /// <summary>The distance from the item to another.</summary>
        [LocalSafe]
        public static float DistanceTo(this Item item, Item other)
        {
            return Maths.Distance(item.X, item.Y, other.X, other.Y);
        }

        /// <summary>The distance from the item to a destructable.</summary>
        [LocalSafe]
        public static float DistanceTo(this Item item, Destructable destructable)
        {
            return Maths.Distance(item.X, item.Y, destructable.X, destructable.Y);
        }

        /// <summary>The distance from the item to a point.</summary>
        [LocalSafe]
        public static float DistanceTo(this Item item, Vector2 point)
        {
            return Maths.Distance(item.X, item.Y, point.X, point.Y);
        }

        /// <summary>The facing in degrees from the item to a unit, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Item item, Unit unit)
        {
            return Maths.AngleBetween(item.X, item.Y, unit.X, unit.Y);
        }

        /// <summary>The facing in degrees from the item to another, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Item item, Item other)
        {
            return Maths.AngleBetween(item.X, item.Y, other.X, other.Y);
        }

        /// <summary>The facing in degrees from the item to a destructable, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Item item, Destructable destructable)
        {
            return Maths.AngleBetween(item.X, item.Y, destructable.X, destructable.Y);
        }

        /// <summary>The facing in degrees from the item to a point, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Item item, Vector2 point)
        {
            return Maths.AngleBetween(item.X, item.Y, point.X, point.Y);
        }

        /// <summary>The destructable's place (its X and Y).</summary>
        [LocalSafe]
        public static Vector2 Position(this Destructable destructable)
        {
            return new Vector2(destructable.X, destructable.Y);
        }

        /// <summary>The distance from the destructable to a unit.</summary>
        [LocalSafe]
        public static float DistanceTo(this Destructable destructable, Unit unit)
        {
            return Maths.Distance(destructable.X, destructable.Y, unit.X, unit.Y);
        }

        /// <summary>The distance from the destructable to an item.</summary>
        [LocalSafe]
        public static float DistanceTo(this Destructable destructable, Item item)
        {
            return Maths.Distance(destructable.X, destructable.Y, item.X, item.Y);
        }

        /// <summary>The distance from the destructable to another.</summary>
        [LocalSafe]
        public static float DistanceTo(this Destructable destructable, Destructable other)
        {
            return Maths.Distance(destructable.X, destructable.Y, other.X, other.Y);
        }

        /// <summary>The distance from the destructable to a point.</summary>
        [LocalSafe]
        public static float DistanceTo(this Destructable destructable, Vector2 point)
        {
            return Maths.Distance(destructable.X, destructable.Y, point.X, point.Y);
        }

        /// <summary>The facing in degrees from the destructable to a unit, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Destructable destructable, Unit unit)
        {
            return Maths.AngleBetween(destructable.X, destructable.Y, unit.X, unit.Y);
        }

        /// <summary>The facing in degrees from the destructable to an item, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Destructable destructable, Item item)
        {
            return Maths.AngleBetween(destructable.X, destructable.Y, item.X, item.Y);
        }

        /// <summary>The facing in degrees from the destructable to another, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Destructable destructable, Destructable other)
        {
            return Maths.AngleBetween(destructable.X, destructable.Y, other.X, other.Y);
        }

        /// <summary>The facing in degrees from the destructable to a point, 0 to 360.</summary>
        [LocalSafe]
        public static float AngleTo(this Destructable destructable, Vector2 point)
        {
            return Maths.AngleBetween(destructable.X, destructable.Y, point.X, point.Y);
        }
    }
}
