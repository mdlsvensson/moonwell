using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>Interact on a unit, an item and a destructable: starts an interaction they hold. A gizmo has Interact as its own member.</summary>
    public static class InteractExtensions
    {
        /// <summary>Starts an interaction held by the unit, and returns it. It belongs to the owner, or else the current owner, and ends with it, with the unit, or by End. Fails in a development build when the unit is not of the interaction's holder class, when its Range is not more than 0, or when it was started before.</summary>
        public static T Interact<T>(this Unit holder, T interaction, Owner? owner = null)
            where T : Interaction
        {
            interaction.Begin(holder, Tracking.UnitKind, owner);
            return interaction;
        }

        /// <summary>Starts an interaction held by the item, and returns it. It belongs to the owner, or else the current owner, and ends with it, with the item, or by End. Fails in a development build when the item is not of the interaction's holder class, when its Range is not more than 0, or when it was started before.</summary>
        public static T Interact<T>(this Item holder, T interaction, Owner? owner = null)
            where T : Interaction
        {
            interaction.Begin(holder, Tracking.ItemKind, owner);
            return interaction;
        }

        /// <summary>Starts an interaction held by the destructable, and returns it. It belongs to the owner, or else the current owner, and ends with it, with the destructable, or by End. Fails in a development build when the destructable is not of the interaction's holder class, when its Range is not more than 0, or when it was started before.</summary>
        public static T Interact<T>(this Destructable holder, T interaction, Owner? owner = null)
            where T : Interaction
        {
            interaction.Begin(holder, Tracking.DestructableKind, owner);
            return interaction;
        }
    }
}
