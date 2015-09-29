#include "include/secp256k1_ecdh.h"
#include "ecmult_const_impl.h"

// adapted from libsecp256k1/src/modules/ecdh/main_impl.h
// return x,y right after scalar multiplication
int secp256k1_pubkey_scalar_mul(const secp256k1_context* ctx, unsigned char *x_res, unsigned char *y_res, const secp256k1_pubkey *point, const unsigned char *scalar) {
    int ret = 0;
    int overflow = 0;
    secp256k1_gej res;
    secp256k1_ge pt;
    secp256k1_scalar s;
    ARG_CHECK(x_res != NULL);
    ARG_CHECK(y_res != NULL);
    ARG_CHECK(point != NULL);
    ARG_CHECK(scalar != NULL);
    (void)ctx;

    secp256k1_pubkey_load(ctx, &pt, point);
    secp256k1_scalar_set_b32(&s, scalar, &overflow);
    if (overflow || secp256k1_scalar_is_zero(&s)) {
        ret = 0;
    } else {
        unsigned char x[32];
        unsigned char y[32];

        secp256k1_ecmult_const(&res, &pt, &s);
        secp256k1_ge_set_gej(&pt, &res);

        /* Compute a hash of the point in compressed form
         * Note we cannot use secp256k1_eckey_pubkey_serialize here since it does not
         * expect its output to be secret and has a timing sidechannel. */
        secp256k1_fe_normalize(&pt.x);
        secp256k1_fe_normalize(&pt.y);
        secp256k1_fe_get_b32(x, &pt.x);
        secp256k1_fe_get_b32(y, &pt.y);

        memcpy(x_res, &x, 32);
        memcpy(y_res, &y, 32);
        ret = 1;
    }

    secp256k1_scalar_clear(&s);
    return ret;
}
