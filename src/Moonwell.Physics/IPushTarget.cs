namespace Moonwell
{
    /// <summary>Reactions to a unit's push, on the unit's class, its components or its buffs; write only the ones needed, as public methods. They run with the unit as the current scope: on the class first, then on the components and buffs in the order they were added, each guarded.</summary>
    public interface IPushTarget
    {
        /// <summary>The unit was pushed: by Push, by Knockback or by a missile. Runs at each push, inside the call that pushed it.</summary>
        /// <param name="pusher">The unit behind the push, as a missile's source; null when there is none.</param>
        /// @CSharpLua.Ignore
        void OnPushed(Unit? pusher)
        {
        }

        /// <summary>The pushed unit ran into something it cannot pass (unwalkable ground, a cliff, a tree, a building, the world's edge): it lost the speed into it, sliding along it or stopping.</summary>
        /// @CSharpLua.Ignore
        void OnSlammed(Slam slam)
        {
        }

        /// <summary>A unit that does not fly, lifted by a push, came back down to its own height (its type's fly height: the ground for most). Named apart from IDamageSource.OnLanded, which a unit class may also write.</summary>
        /// <param name="speed">How fast it was falling when it came down, in game units a second.</param>
        /// @CSharpLua.Ignore
        void OnGrounded(float speed)
        {
        }
    }
}
