namespace Moonwell
{
    /// <summary>Map-wide reactions to damage, on the game's components (every hit) and a player's (hits where the player owns the source or the target); write only the ones needed, as public methods.</summary>
    public interface IDamageWatch
    {
        /// <summary>Before armor, for any unit taking a hit: the hit may change its amount, its types and the target's armor for this hit.</summary>
        /// @CSharpLua.Ignore
        void OnUnitTaking(Hit hit)
        {
        }

        /// <summary>After armor, for any unit taking a hit: the hit may change its amount.</summary>
        /// @CSharpLua.Ignore
        void OnUnitTaken(Hit hit)
        {
        }

        /// <summary>After armor, when the hit would kill its target: the amount may be lowered, or <see cref="Hit.Spare"/> called.</summary>
        /// @CSharpLua.Ignore
        void OnUnitDying(Hit hit)
        {
        }

        /// <summary>0 s later, when the target's life has dropped; <see cref="Hit.Killed"/> tells whether this hit killed.</summary>
        /// @CSharpLua.Ignore
        void OnUnitHurt(Hit hit)
        {
        }

        /// <summary>0 s later, once for each source that landed hits in that moment.</summary>
        /// @CSharpLua.Ignore
        void OnUnitBurst(Burst burst)
        {
        }
    }
}
