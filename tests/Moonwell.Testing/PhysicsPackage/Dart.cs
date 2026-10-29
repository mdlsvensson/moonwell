using System;

namespace Moonwell.Testing
{
    /// <summary>A missile the flight cases make, flying through the ground and units: a case sets its settings, and it counts its OnExpire and runs an action a case gives it there.</summary>
    public sealed class Dart : Missile
    {
        /// <summary>How many times OnExpire ran.</summary>
        public int Expired;

        /// <summary>The owner that was current when OnExpire last ran.</summary>
        public Scope? ExpiredWith;

        /// <summary>An action OnExpire runs after it has counted; null for none.</summary>
        public Action? Expiring;

        /// <summary>Makes a dart that flies through the ground and units, so that the flight cases see its motion alone; the ground cases use Ball.</summary>
        public Dart()
        {
            GroundContact = Contact.None;
            UnitContact = Contact.None;
        }

        /// <summary>Counts the expiry, keeps the current owner, and runs the action.</summary>
        public override void OnExpire()
        {
            Expired++;
            ExpiredWith = Scope.Current;
            if (Expiring != null)
            {
                Expiring();
            }
        }
    }
}
