using System;
using System.Collections.Generic;

namespace Moonwell.Hosting
{
    /// <summary>The game as the areas package sees it: regions and the events of units entering and leaving them, the units in a rect, and the events that say a unit died or came back. Types are the game's numbers.</summary>
    public interface IAreaWorld
    {
        /// <summary>Makes an empty region.</summary>
        region CreateRegion();

        /// <summary>Adds a rect to a region. The game rounds it out to whole 32 cells.</summary>
        void AddRect(region r, float minX, float minY, float maxX, float maxY);

        /// <summary>Adds the 32 cell that holds a point to a region.</summary>
        void AddCell(region r, float x, float y);

        /// <summary>Registers the two triggers of a region, one run when a unit enters it and one when a unit leaves it. Inside either call, <c>EventUnit</c> is that unit.</summary>
        void Listen(region r, Action entered, Action left);

        /// <summary>The unit of the event being delivered: the one entering or leaving, inside the calls Listen gave the game, or the one that died or came back, inside the calls ListenDeaths gave it.</summary>
        unit EventUnit();

        /// <summary>Destroys a region and its two triggers.</summary>
        void DestroyRegion(region r);

        /// <summary>Whether a unit is inside a region.</summary>
        bool IsInside(region r, unit u);

        /// <summary>Adds the units whose centre is in a rect to a list, in the order the game gives them.</summary>
        void UnitsInRect(float minX, float minY, float maxX, float maxY, List<unit> into);

        /// <summary>Starts delivering the game's unit death and hero revive events, for every player slot, once. Inside either call, <c>EventUnit</c> is the unit that died or came back.</summary>
        void ListenDeaths(Action died, Action revived);

        /// <summary>Whether a unit has a place in the world: it is alive, not hidden and not loaded into a transport.</summary>
        bool HasPlace(unit u);
    }
}
