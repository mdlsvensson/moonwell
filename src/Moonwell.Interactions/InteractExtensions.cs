using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>Interact on a unit, an item and a destructable: starts an interaction they hold. A gizmo has Interact as its own member.</summary>
    public static class InteractExtensions
    {
        /// <summary>Starts an interaction held by the unit, in a scope, and returns it. It ends with the scope, with the unit, or by End. Fails in a development build when the unit is not of the interaction's holder class, when its Range is not more than 0, or when it was started before.</summary>
        public static T Interact<T>(this Unit holder, T interaction, Scope scope)
            where T : Interaction
        {
            interaction.Begin(holder, Tracking.UnitKind, scope);
            return interaction;
        }

        /// <summary>Starts an interaction held by the item, in a scope, and returns it. It ends with the scope, with the item, or by End. Fails in a development build when the item is not of the interaction's holder class, when its Range is not more than 0, or when it was started before.</summary>
        public static T Interact<T>(this Item holder, T interaction, Scope scope)
            where T : Interaction
        {
            interaction.Begin(holder, Tracking.ItemKind, scope);
            return interaction;
        }

        /// <summary>Starts an interaction held by the destructable, in a scope, and returns it. It ends with the scope, with the destructable, or by End. Fails in a development build when the destructable is not of the interaction's holder class, when its Range is not more than 0, or when it was started before.</summary>
        public static T Interact<T>(this Destructable holder, T interaction, Scope scope)
            where T : Interaction
        {
            interaction.Begin(holder, Tracking.DestructableKind, scope);
            return interaction;
        }
    }
}
