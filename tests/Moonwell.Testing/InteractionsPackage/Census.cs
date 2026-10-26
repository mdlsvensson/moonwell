using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>An interaction the interactions cases start on a unit to see which things a class of other takes in: it pairs with the things of class TOther within 300, checks at every step, and keeps the others it met, in order.</summary>
    /// <typeparam name="TOther">The class of the things it pairs with.</typeparam>
    public sealed class Census<TOther> : Interaction<Unit, TOther>
        where TOther : Actor
    {
        /// <summary>The others it met, in the order they met.</summary>
        public List<Actor> Met = new List<Actor>();

        /// <summary>Makes a census of range 300 that checks at every step.</summary>
        public Census()
        {
            Range = 300.0f;
        }

        /// <summary>Keeps the other.</summary>
        public override void OnMeet(Pair pair)
        {
            Met.Add(pair.Other);
        }
    }
}
