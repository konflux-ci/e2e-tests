
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: klusterlets.operator.open-cluster-management.io
spec:
  group: operator.open-cluster-management.io
  names:
    kind: Klusterlet
    listKind: KlusterletList
    plural: klusterlets
    singular: klusterlet
  scope: Cluster
  preserveUnknownFields: false
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          description: Klusterlet represents controllers to install the resources for a managed cluster. When configured, the Klusterlet requires a secret named bootstrap-hub-kubeconfig in the agent namespace to allow API requests to the hub for the registration protocol. In Hosted mode, the Klusterlet requires an additional secret named external-managed-kubeconfig in the agent namespace to allow API requests to the managed cluster for resources installation.
          type: object
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
              type: string
            metadata:
              type: object
            spec:
              description: Spec represents the desired deployment configuration of Klusterlet agent.
              type: object
              properties:
                clusterName:
                  description: ClusterName is the name of the managed cluster to be created on hub. The Klusterlet agent generates a random name if it is not set, or discovers the appropriate cluster name on OpenShift.
                  type: string
                deployOption:
                  description: DeployOption contains the options of deploying a klusterlet
                  type: object
                  properties:
                    mode:
                      description: 'Mode can be Default or Hosted. It is Default mode if not specified In Default mode, all klusterlet related resources are deployed on the managed cluster. In Hosted mode, only crd and configurations are installed on the spoke/managed cluster. Controllers run in another cluster (defined as management-cluster) and connect to the mangaged cluster with the kubeconfig in secret of "external-managed-kubeconfig"(a kubeconfig of managed-cluster with cluster-admin permission). Note: Do not modify the Mode field once it''s applied.'
                      type: string
                externalServerURLs:
                  description: ExternalServerURLs represents the a list of apiserver urls and ca bundles that is accessible externally If it is set empty, managed cluster has no externally accessible url that hub cluster can visit.
                  type: array
                  items:
                    description: ServerURL represents the apiserver url and ca bundle that is accessible externally
                    type: object
                    properties:
                      caBundle:
                        description: CABundle is the ca bundle to connect to apiserver of the managed cluster. System certs are used if it is not set.
                        type: string
                        format: byte
                      url:
                        description: URL is the url of apiserver endpoint of the managed cluster.
                        type: string
                namespace:
                  description: 'Namespace is the namespace to deploy the agent. The namespace must have a prefix of "open-cluster-management-", and if it is not set, the namespace of "open-cluster-management-agent" is used to deploy agent. Note: in Detach mode, this field will be **ignored**, the agent will be deployed to the namespace with the same name as klusterlet.'
                  type: string
                nodePlacement:
                  description: NodePlacement enables explicit control over the scheduling of the deployed pods.
                  type: object
                  properties:
                    nodeSelector:
                      description: NodeSelector defines which Nodes the Pods are scheduled on. The default is an empty list.
                      type: object
                      additionalProperties:
                        type: string
                    tolerations:
                      description: Tolerations is attached by pods to tolerate any taint that matches the triple <key,value,effect> using the matching operator <operator>. The default is an empty list.
                      type: array
                      items:
                        description: The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.
                        type: object
                        properties:
                          effect:
                            description: Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.
                            type: string
                          key:
                            description: Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.
                            type: string
                          operator:
                            description: Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.
                            type: string
                          tolerationSeconds:
                            description: TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.
                            type: integer
                            format: int64
                          value:
                            description: Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.
                            type: string
                registrationImagePullSpec:
                  description: RegistrationImagePullSpec represents the desired image configuration of registration agent. quay.io/open-cluster-management.io/registration:latest will be used if unspecified.
                  type: string
                workImagePullSpec:
                  description: WorkImagePullSpec represents the desired image configuration of work agent. quay.io/open-cluster-management.io/work:latest will be used if unspecified.
                  type: string
            status:
              description: Status represents the current status of Klusterlet agent.
              type: object
              properties:
                conditions:
                  description: 'Conditions contain the different condition statuses for this Klusterlet. Valid condition types are: Applied: Components have been applied in the managed cluster. Available: Components in the managed cluster are available and ready to serve. Progressing: Components in the managed cluster are in a transitioning state. Degraded: Components in the managed cluster do not match the desired configuration and only provide degraded service.'
                  type: array
                  items:
                    description: "Condition contains details for one aspect of the current state of this API Resource. --- This struct is intended for direct use as an array at the field path .status.conditions.  For example, type FooStatus struct{     // Represents the observations of a foo's current state.     // Known .status.conditions.type are: \"Available\", \"Progressing\", and \"Degraded\"     // +patchMergeKey=type     // +patchStrategy=merge     // +listType=map     // +listMapKey=type     Conditions []metav1.Condition `json:\"conditions,omitempty\" patchStrategy:\"merge\" patchMergeKey:\"type\" protobuf:\"bytes,1,rep,name=conditions\"` \n     // other fields }"
                    type: object
                    required:
                      - lastTransitionTime
                      - message
                      - reason
                      - status
                      - type
                    properties:
                      lastTransitionTime:
                        description: lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
                        type: string
                        format: date-time
                      message:
                        description: message is a human readable message indicating details about the transition. This may be an empty string.
                        type: string
                        maxLength: 32768
                      observedGeneration:
                        description: observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.
                        type: integer
                        format: int64
                        minimum: 0
                      reason:
                        description: reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty.
                        type: string
                        maxLength: 1024
                        minLength: 1
                        pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
                      status:
                        description: status of the condition, one of True, False, Unknown.
                        type: string
                        enum:
                          - "True"
                          - "False"
                          - Unknown
                      type:
                        description: type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
                        type: string
                        maxLength: 316
                        pattern: ^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$
                generations:
                  description: Generations are used to determine when an item needs to be reconciled or has changed in a way that needs a reaction.
                  type: array
                  items:
                    description: GenerationStatus keeps track of the generation for a given resource so that decisions about forced updates can be made. The definition matches the GenerationStatus defined in github.com/openshift/api/v1
                    type: object
                    properties:
                      group:
                        description: group is the group of the resource that you're tracking
                        type: string
                      lastGeneration:
                        description: lastGeneration is the last generation of the resource that controller applies
                        type: integer
                        format: int64
                      name:
                        description: name is the name of the resource that you're tracking
                        type: string
                      namespace:
                        description: namespace is where the resource that you're tracking is
                        type: string
                      resource:
                        description: resource is the resource type of the resource that you're tracking
                        type: string
                      version:
                        description: version is the version of the resource that you're tracking
                        type: string
                observedGeneration:
                  description: ObservedGeneration is the last generation change you've dealt with
                  type: integer
                  format: int64
                relatedResources:
                  description: RelatedResources are used to track the resources that are related to this Klusterlet.
                  type: array
                  items:
                    description: RelatedResourceMeta represents the resource that is managed by an operator
                    type: object
                    properties:
                      group:
                        description: group is the group of the resource that you're tracking
                        type: string
                      name:
                        description: name is the name of the resource that you're tracking
                        type: string
                      namespace:
                        description: namespace is where the thing you're tracking is
                        type: string
                      resource:
                        description: resource is the resource type of the resource that you're tracking
                        type: string
                      version:
                        description: version is the version of the thing you're tracking
                        type: string
      served: true
      storage: true
      subresources:
        status: {}
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
jǺyzKyejeWgЦfW'6cCW76PWFFFFF3v&BV6gBvVC&vVVB S&V6W7FW"vVVBvVB ЦfW'6cC6W'f6T66V@WFFFSW7FW&W@W76S&V6W7FW"vVVBvVB vUV6V7&WG3S&V6W7FW"vVVB֖vRV7&VFVF2 ЦfW'6&&2WF&F泇2cC6W7FW%&PWFFFSW7FW&W@'VW3w&W3"%Т&W6W&6W3'6V7&WG2"&6fv2"'6W'f6V66VG2%ТfW&'3&7&VFR"&vWB"&Ɨ7B"'WFFR"'vF6"'F6"&FVWFR%Тw&W3&6&FF泇2%Т&W6W&6W3&V6W2%ТfW&'3&7&VFR"&vWB"&Ɨ7B"'WFFR"'vF6"'F6%Тw&W3&WF&F泇2%Т&W6W&6W3'7V&V7F66W77&WfWw2%ТfW&'3&7&VFR%Тw&W3"%Т&W6W&6W3&W76W2%ТfW&'3&7&VFR"&vWB"&Ɨ7B"'vF6"&FVWFR%Тw&W3"%Т&W6W&6W3&FW2%ТfW&'3&vWB"&Ɨ7B"'vF6%Тw&W3""&WfVG2泇2%Т&W6W&6W3&WfVG2%ТfW&'3&7&VFR"'F6"'WFFR%Тw&W3&2%Т&W6W&6W3&FWVG2%ТfW&'3&7&VFR"&vWB"&Ɨ7B"'WFFR"'vF6"'F6"&FVWFR%Тw&W3'&&2WF&F泇2%Т&W6W&6W3&6W7FW'&V&Fw2"'&V&Fw2%ТfW&'3&7&VFR"&vWB"&Ɨ7B"'WFFR"'vF6"'F6"&FVWFR%Тw&W3'&&2WF&F泇2%Т&W6W&6W3&6W7FW'&W2"'&W2%ТfW&'3&7&VFR"&vWB"&Ɨ7B"'WFFR"'vF6"'F6"&FVWFR"&W66FR"&&B%Тw&W3&WFV62泇2%Т&W6W&6W3&7W7F&W6W&6VFVfF2%ТfW&'3&7&VFR"&vWB"&Ɨ7B"'WFFR"'vF6"'F6"&FVWFR%Тw&W3&W&F"V6W7FW"vVVB%Т&W6W&6W3&W7FW&WG2%ТfW&'3&vWB"&Ɨ7B"'vF6"'WFFR"'F6"&FVWFR%Тw&W3&W&F"V6W7FW"vVVB%Т&W6W&6W3&W7FW&WG27FGW2%ТfW&'3'WFFR"'F6%Тw&W3'v&V6W7FW"vVVB%Т&W6W&6W3&ƖVFfW7Gv&2%ТfW&'3&Ɨ7B"'WFFR"'F6%РЦfW'6&&2WF&F泇2cC6W7FW%&PWFFFSV6W7FW"vVVCW7FW&WBF֖vw&VvFR6W7FW'&P&V3&&2WF&F泇2vw&VvFRFF֖'G'VR 'VW3w&W3&W&F"V6W7FW"vVVB%Т&W6W&6W3&W7FW&WG2%ТfW&'3&vWB"&Ɨ7B"'vF6"&7&VFR"'WFFR"'F6"&FVWFR%ТЦfW'6&&2WF&F泇2cC6W7FW%&T&FpWFFFSW7FW&W@&U&Vcw&W&&2WF&F泇2C6W7FW%&PSW7FW&W@7V&V7G3C6W'f6T66V@SW7FW&W@W76S&V6W7FW"vVVBvVB ЦCFWV@fW'62cWFFFSW7FW&W@W76S&V6W7FW"vVVBvVB &V3W7FW&W@7V3&WƖ636VV7F#F6&V3W7FW&W@FVFSWFFFFF3F&vWBv&BV6gBvVVCw&VffV7B#%&VfW'&VDGW&u66VGVƖr'p&V3W7FW&W@7V36W'f6T66VDSW7FW&W@FW&F3VffV7C66VGVPWFR&RV&W&WFW2g&W&F#W7G06FW'3SW7FW&W@vSV7F7G&&Vv7G&FW&F$6#ScfccSFfcc&CS#6S#Cff#336Cs3fcfVcSCs3C6#3pvUVƖ7dE&W6V@&w3"&Vv7G&FW&F" &W7FW&WB "F6&RVFW"VV7F ƗfVW75&&SGGvWCFVF66VSEE0'CCC0FFV6V6G3 W&E6V6G3�&VFW75&&SGGvWCFVF66VSEE0'CCC0FFV6V6G3 ЦfW'6cC6V7&W@WFFFS&&G7G&ևV"ַV&V6fr �W76S&V6W7FW"vVVBvVB �GSVPFFV&V6fs%'fg3&f&vFT'35&6d6u'35&64tt6'ƥ&tcDvf6U3&$W#DdׅeWVTUVŤedev$UeVvDWDԅ$FdSe5Fg%F%%c&Ev$te6wde%64tVdVׅUw4u&F5#%c'G6#%'EF%fudUdVeSUdedėu%ueedUDFE5&%3Futd%DeUVƆ5u$Uev$U5Vettd6EtԄ'dU#%&V%ׇv36wuEfveVe$vGu5tdFUc4$DedUVUu%wC5de5g5sEdwG4'5VeDeeS$5FŤe&t$5EDS啤U%fׄefׄuV$ef4gVfTEEƅ4TׄtduVUdwedee4uuEe6DeV%W5uc%eV%v&ׄf4e$vS&ӓUFCFg%F%%fTD%5VEf#ED֦ƅSFƷtTeeeEUwuĤsU5f5Vd$deu&$'UVd'V&Ue&d%c5dDTUufc5cfTfvsTC4%EEUgv3$dfԄ%FcE3%f&%%EwUSWV$$ECe5c%d%uevV3Cg6&ťw4e$Eef3Ueee6&v#DU7V4W7vDDF$s37Tg5EEdeEeDuVdfDfE&efeueU44TdDfTtdUfee'DdvV5%T5vƄ3WDcD&CfVևf5e%u6VeDe5fFtU%WŤ$u%v4EdWEEe&C%duU&ׇ7ffŧ%fEdUf6&&'E#c6C$VfUefvħUfUUFĥfFė%&fuutU5eeT6g64s$%VSTFDedftֳeUFƳ#eEtwUcEC%W4$UgvfuEd$defTvUSdTutv36Fg56Ɩ&euf5tdUUf&ee&օVdeeuuVUedew5Udeu&W5eee5vDUeUSV$fTeVeEDDVħ$TUeW5vu&5&EEUSUg6$U65%EcguEguEE6VTD5fs$wSuCu&Ue$54dWF4STuw%eeef$%%uUeddde6eeWeDUdefDfF5SDuTVdDdeeFĦdu$U&Vg&DVdu'EUwG54S&du&ccE$SUFdVeufvFGuD%suef5fԧ6E%E&SuD5ugETWuuC$eUwFdWE37uv$eEdtUFƄ6$SW%6dvdDtSdDfSUUs4ėv%eUFuTeUEEtwUw#%gFTg$S%SW$ՄE&%S%5De%TWUUsV3E4dwC%Cev%veV$vud6%ed&ĥu&V%eUU$&$6c5#TֳevF#CS&G&f&5tfDeg%UsfTS%w%VteVV$Teee6u%ee&%stSUeEE&W%dU$3%%dfF#&3%uԥ$VD&us5S$D%g%VUvFť4dfĥU7&USeVu#E7uTfF4T5dWեwD#dԇ&ueDf$uf4DdeSTfƆ6c6#5$W#DeeuCE%5feSf6Wu&e5W#DdvDTԅ$W#UwEu4ddvE$eEfg5#eFu%ecDdvDW&4SUEeweeee5$dWu&&c%$5D'5eWEeSDguedu5E%tgD%5VEf4WՄ$V'%&ĥ5eEeedtd4UdDdEUefufFCdegvSu6%EeutfGUdS#vĤuvĥeeduU'6UguWF%VVfDd֥fD4SFgeuwF#%'EEetddu3$E6%v&ׄUw5uduVeDdefƇ5TdefeVuD3$Ueu%DSdee5fedee5twVet%TTVŤFg%&%dUuVgu$uddWETEDEe'Dvts5de&v4sUedeeu%df%dS#EEeuvfօDEfGFTwdUeuwC$d64gEDdtTEEƕ'gŦe%f$W&'UeSD#5'uV4e6eeeUeduTdWu&ĥ%USg5WW$U5VdtegEeUfcE4$Ee&efvDօeS%$WF$u#EU4tV$&EEf3Wwu5&&uw3$u'&4TEddfC5dDTEEs$dufEE%EfUe$6$weFVTdVDwF%eUSTSGv4Dfdu'uudtՅW5cDSUe4ַ&VV#f$VF%SW%'4ugDTŤu#utufĥ&wvU4eWuFĥeEg5EetgU&efׇfFdwefDd$%fEUfvĤdSTuesTuvħ&DEe%veve3Ef'f$eEEuudFTvegfC$ef&Uw%e56%cWuFCFdׅF4$vCE4u%fVģdDufgDtddu#UŤuc%d&4VĤEede&f&cEdu$uv4wDD$SWVW6%%c$5vdVtudfDDugE6WEeDUSUVe&$TVƇ$dW#5F'UDd6ƅtu$VduVee6%wweee5u6UV$gee&euE%5eW5dedu&54u&VSuVeevC%UwDufu&5eVDDue6UV$gee%5&ħEdudu&'3U3Ev4EDES%FFTSG6dSTGvCWu&Ɣ%$eee53הteEc&wuDeef&$U5Vgv&uf%edgu%Ww#6%ec5dV7C%dVTu4eDfdv֥#%'55euF%Ueu'5fŤ3%eF%ddUD#6EE'eVET5SUV%v%uUe$v$d#UeUdĤv$vƕceW#F6&$55SV$ddu#%VW3EuVU4WvTeF&dTUDUegUWEEeFVEuVE&ֵuwF3EeFdDS$g%eE%ufvŤSWDS&UEV&udU55tW$sV#UEsuvԤ6%FC#dD'uCv$Tdu'STdWfF%dsTddeFĦ5uUcEdfD7&%eeUe$UFu'gwvEtWVdgC5uf&&d&VTfv5$w3f$tEe6VdU5TSUU6FeDEeSVUee&%EtUSetef$u$5dsC%U&%fdeVev%$se3$g6$v5#'uDCueDSWewD$%dv喤&f%&#5$W#DeeuCE%5feSf6Wu&e5W#DdvDTԅ$W#UwEu4ddvE$eEfg5#eFu%ecDdvDW&4SUEewefe5$dWu&ed$5D'5eWEeDg%esudFDU%tVD%5VEf4WՄ$V'%&ĥ5eEeedtg%VdDdEUefufFCdegvSu6%EeutfGUfVėdUduvĤuvĥeedueUguWF%VVfDd֥fD4S4te%sUueC$fTfc6Dvf%'EF&EsFEfDfFeDdEF%ef$EeUedueSU%d$S5%ede$vFDeVeUtwVte'5VeEdeVƆFDfueGuFg%&%dUuVde%f4vWusC$&ƅe4$e&5&5eee5&euv&$ŖcEtdUv%EguDtF$t&cgfUTE&ƅV%fv3fwF%SS6FTe%DeeW6e6Us6et5eu&65dD$eVTUewUwD&uEWE%fuUf64gUEVefgVfŅ%VS'et6$sfVVv&Dw$wwEC$64e5ewVDvVeeF#V5TfGDWU$cuSGDU%5eWvdUT'd5FS&ťDD%e#4UdwG6wvDF#Vf$W&$d5V3Sv4vtׇe'5#%e6$Uce'4SU&ƵtweuDf%$FggufŤvf4D'uefg&4tet֦5F'ŧFTf4vsufCS$DSdFEu6DfG&TVEgƄe3%deFcefŤSW5vfc6dEFftSGc&EtV'&ֆUff%SԄ$W'6TtdsUdTWUD$uFUƕ'eEUsW&F4U%eewEV%UFUEUcDevĤU$S'$$fTE%Vv%%fĥgUVW64eFVħ%SG$ud54dcedUguDe$eDF$SS5dwe$v$wSTEfdSUfDS$EsW4fEeFEed5eedu$eUeUeSUtfEE&SUwDv%Sef%eDdUUfuVuVeddfׄCe&ĥEEUeteU&F%UfE5tfEE&WuVf&eueE%Tdw3UŤeD4WœtgEF%u$u#5FƆSD5&S&GuDt4W4gdwDuVĥfTTeeUev&g%&DTuF4法V4euD%dUuVduuDffEFwgu%SUu'eE#TV4E%4֤cFW635Fe$VeDDgvťU#wueUƥf7uvfF6Ɔ#cEdDFEuSUggfDttו&Tַc'Eu#gUuWF%DgUSe6$eveTSUVGuTdeFF5ef#$evƅudw%c#tUf%f64ug&UeDdwuTSuUE%v#%F#EdeuFU'UuD'6CWvDvƅeu#esVEtf$D&4u%U'&FGDVFfWuUfv6F%$ĤfDtS5teE#'wufׅ5%ugVsUvUee$f%vc#&VGVf&׆c#4tV%te36eg6UegUEVD&du4%dvfEf4EU%gFU%'$sfv%Wwv$E&fe3U6f֦eVGttv$Ve#uUuEu'U&F%FUfW5ege&EcuSuUdu%Dfudeu3Dԅ$WdudwE%Wufu%wSdŤee$W#DdgfDWDԅ$eWuV'5CeFf$dVG5$deuVW#Ddu3%f$W5#6s%UD$ԧDsU%tu$UwDv&f$VƕdUvf'FTfF%$eee53הteEc&wuDeef&$U5Dfed6SU6$eUe$ufĥ&4T&uw$6eV%FfgfC$uD$f׃fƅ5vDe6EVfE6DeV$SvVwc'3$dVtwEsVEU6FֵCvce&dF$%dsV$uvffEwuud6օvVţdUC$v#6c6D$te'F4eS%ef5FŤEeUeVuce5ed$SU5&œdD%5&edUfVddfCFFDeVeUtwVte'5VeEde5FDtdcW#eTfŤuUfe5&%3Futd%U$uvԥuf%fքEgv5vf'eecEtSte%'evfueu64gEwFVef%dUEf5etD'efEudS4te%sUuet&TfV&Ewv3EF%tvwt'V4deDeev5Ff$TEc4$5dwD&W&ӖׄCCg%&ĥ5eg5UeSU$deu%E%%eDdSd$fvDU%%fuUe&5SGeWD&%UDdegU5D%6EfCEe'c4'5Vedfdf&%tg6$sTufD%'&TE%5#ewGu$%eDgEdUduetEd4'5'Guv4f5f׃ƅuTtWF%Ec%EfE6VWFv%wdUfEugU6ׅvևcD$g64S5&Vguf$u6%eWEUefdev%Ed'edgUfdFuttgV$T$$fC66WDEfԄ%ds53e%D&ׄŖ3UC'F$SV&օUDufԗuFׅc'w5udDdew3$$内E#63DvCe6&c%7euUS%&eCUEU$v%tVEf&ceS#U4tf$%FeU'56F4vS5UwTedf7%deeeDvC64edfcV3$U4u#Uuf$tefdf$54eddՄ&dd%%dtv4SU&dwFd4E&%&5ev4U#C%f6Ԥe%F%W#Wuc%'U6fwSuVeeweUeugDUtu&fduTdgESv&5CdU&UD$vTeeUfwefׄCe&ĥEUwDufu&5eV%wDvgESv&5Feu#&ťDԄ'5v5dte%D&%u&Fť53%$w$WTUvdeuD&vwce5cge&UGu#$u&ťDԄ'5v5dte%D&%u&Fť53%$w$WTUvdeuD&d5cguDt4W4gdwDuVĥfTTeeUev&g%&UUDD4TF4sF5U6UteVUUe$&6gE&3De4eE&%FedTDtfeu%WU6TuuuD6&eVudeWegeD#uW7eWEecuVŦ#$E&ŅS5Weuef&&ŗuDdudf5c%$euDfTSet&EVׅ$u%ff$$uc%de&f#6EdfG5fŤufŅU#EwGuf6dUSF%eg&Dd%4Eedu$d֦ĕc#Uu'UV&EfE&6ԧ5eE&c6vW5fG6$Eet#E7ԗv׆%Ece#Te%Uvf&Wuve&#W$%fTdVօvTu5u%4TU4SUEF4ֳW&6E&eus%tV4e$%DeUUedEd45V%%3WUS$eeV6def5&ץFӕ5#5%7fF6$W6Vs7d56&'efV$UDdvC$WV$dd7&Cveu&ևuS'5et&4e&ws%Ťv6W$U%c4'EUw$Tԅ$W#VS%VeFf$dVG5$deuVW#Ddu3Dԅ$WdEVf6'%5feSf6Wu&e5W#DdvDT$WEegVd$dg&$U&USdv$e%&U%uf3U%u$&v%$S%df5dTc'5ue%e%E4SUUfdd55&uUwDe%WEUedUdef4TEeUwDuvŤe&v$dew4%&5&5eec&#e&U$v5dg%vƕeESvŦdd%$Uee5%e5&ĥ5$uD'ude%4TefEV5Fg%%Uc4'UUW5deUFUu%gVVC%edՄe%ddw4e%4TDeeUsuFĤE%Td5EwD&'5v%eDee'C6UdUvfVŧuwFc%gEVv&ׄf4fEU6DԆVuvEUU'eVDDegF$vdsuVvT%tu&ffC%SVfCUC%3%f$W&'UeSD#5'uV4e6eeeUeduTdWu&ĥ%USg5WW$U5VdfTtE6eeWuwFudv4E$VƷV%u6e&4VEEeg%UwuvE6FdS%eee5efec%%%764tV35fţUVcF5tהWVօW$ve'EVVTF$dSTUF&cU7&4eg%FF%g5vdu%UuDeFVTeD%fEUFfħu'3dSSe%DES7eefedES#EedU6ƆvC5uUtu%fef$UcE5c5EeeUdťsEu'UV$$veDftו&%dԆs'FdF$dUeu&ť&Ħc6C%vCEԧ5%D#5ewE$SWUU#'CUUfD4d&d%v%VGv3UF&5w$t$vUfeUe%vC%'DՅ&4UUfC&dՄ'eFGvEegE5$wuEU&TugDD%ufׇEU$6&ׅF$suEUgfCE64Մ$eEU6ĥf4e$C5%t׆VDU$դDfֳU6%wwuFg65u%e&cVuvGuEdwEcC6UtUVWUScF$fETUVduVee6%wUu$dWu&F%$evg5%e55Ee%U5SUeu&e%c%%veTfŧ%S'uVŗtTeeu$UUD4deu&ť54$EDgEsVUVfeFe%c4$5ded&'5vfdEUfEde&e%eUUw54deU&Ŧ%dUeeg5Vee%w5fuvwf4u#5#tdeՄVŗuwDfUtdV$ee%cEvcuFU5eu$eUe&g5&ťU4Wc&TtduEdť5UDeu%teħuU5$ud4TvUv3gEU65uD$eu6EUeeD'5VEf$WwEUwufdT4u$fCfT6sC5FeUfU'SDDe'V$Sdwsc#Wue&4T#UUs&ħ%6ĥ%$5g5tffd#TeESv$EU'gfD4eUfwwuw3$&TE&&ŤEvcu65f%UsF&g&$F6VVdDedVEfDVgFTwdUUusC$EVdtWSEgf$wFCvԥV%UfĦEUSSUv4fvftg&T%V&uw56'6$f&&SVvS&%S6SfeWfDfUSU4tdֳGu#%%v4VF&5VefUe'%VceUsFt56tw7vGvSu%UVST5fŗefue6U%EgUefeUEWccEDu$T&Ue%e6eeeW%5ԥe6VEv%u#%6ĥST5ew6ffd#TeES$D'7guuegV4tevVŧuD3%V$tgDU5eWUuf%cFdv&eeEWE&'Eed6$Te%U5vF%gUD&SguUdU6f$u#UVFetgfք&UeevTegU&ƖVWVƄ6FD$uev$UFC4egUdee&5eUUwFFĥuf%EfefDEfuF%dFvħUVE'eEVGvFĤfWtVSvCWF7f%uSW%ewU'ԥfֶdf3VEf&DD%F$fuEUVg5FԆ74tdfEucu%T5vFE4W%vTuSeV$g%%3ueeUUd$UedַuVTfF$Vwe$euesVee&5eecFee&5eUUwFFĥuf%Eegvv%deef4vw66%&4sV&ĤEVvUtg5vgufd5EeV$g5UefV$v$W%c'uUe'63&$&wf&u%SGeSV6ueD#%uDuSWSFWDuF$Udv5D%5VEf4WՄ$V'%&ĥ5eEeedu%fU&FƇ$ugUVŦ&ETu%6$u'UcUW7uFƄTduDf6W&ed5F%duDe&ESFUtW&u%ufgtede5fVvFedTֵEeD64e%fef'UvFVFF&'Tgdt6憕dvsVFf$T&&'Ef5uetSef5UDu$UeƵ4SWEeV$ֳUv$svƄvUfUdWS'3FTSU6ee4v'uTE%UuEe6ev&%5fDdԆ5&cEֳV6eVEtu%uTFԥFDvu$wGEVDuESe6ԆES$TE53Ef#Ed֦eSTSGtDf4w%ttv&ֆVTd%UD'C5V6g$D%dTUCDUFU4STwDWw$dׄFcŗ$T54#ettWdffFtWVCEUSV4eԅ%fVvwtE%V%EF6$tuU%u#'Efĥ6tu%5eeW&dWDW#Ddׅ&'%UvE$eEfg5#eFu%ecDdvDW&#5$W#Ded&ėv$S%STuew56ħ&$U%%fĤuDdvDWDWEUewVF5dWuFUUf6g%&UEf5S'EvUdWufe%dEf3fw6ƅU#'׆TtUeefׄWUdDEd5eeeuEdg5&C4%ewEuEe%e&4UcEfeu$ddU%v$gUcedeTEeUwDvFŤf$U4e%VffDfD%wdsEvvefօEFg5#FTw$ge#De'DW5cuwc%tdutu$uDg5%e5&ĥ5%eUguff4VEegŤsVef5%eSF'$Մe%tu%fdfC%eVDdeUf3wVgfƆTeeeUfĦD%defdevVuFĥedeeu4dWEv$e%VŦFg5&%dUuVd#%dee4f5S'F64dvVŧuD3%Ve5eUfC4deU&ť5eUdedfg$D$ueWeU%deD'6SuVĥ5eW5eeueUV&Uf&ufF%SdeeED%c&uuUfŅVe%Vƅte'5veU%dgScDt&d$U%f3ueW&憵&GFƆfe$eU7fudEeTdewuVťuvev$vS'Fԗ4׆S&VTd%&uFSG6eSGc5CG$d&&ē%Es&eUDeu3WV'%TgvVdueE%%V7v6w5De54dfCte&6$d%w56ťeDtVEw&Vԥe64UfƄdGUVf#eee5S$&7eVVֆf6DT$VFuUeV$%$eUeufUeFUsVSFӖ#$V%t%Fƶ&քDDu4StVťcUEDeuS%%uV$SUUs5EudEUƄFĥvd4'FCEUWwtdWEF׆Fĥfgd5$USUU&dֳUwv%ew&%SUu'TtcTDft'F$ťUv6C#%D5#&F4WuftuduUuc%%%Ee&c%tSUUfFFg%&4'Uv%u6%f$W&eUeSUtfEE&SUwDv%Sef%eDdU'DuvEVefׄefׄC$e6tu$Ue56ץg&5ffv$V$SSeV$uVťevDdEded6ėu%ucuD%uwdEeDd5cd3euE%$U5Uf5eev$Uev$V5dD$EefE&$U$SVE6Udu3edg&EwCE#%fTVUeVV$fTeVUdedde&4VEE'#5fuSGVe#5$5cSUWv$eEeu'wGUVeUedV$$tuTefŤUsV&g%6%tu$Vdv4dgEsVUVfeFF%cdu5Ed%wC6VSc5uwFc%%v7EteED'vEe$4TUdw5egUEed5vW%eVeUVFtVf&dD$u'Efe5tee%vVG5f%SWuFƆԥFUeSTV$fTeV%fded6uFeu&FeuFeu&&d5EUf$ueF%f5UvGv&$Vee%fuUw7eU%deD'6SuVĥ5eWUDeueUV$U5UD&tcefe4U%EDfWuvV$gvcEDfEvdUeewtT5eV%tf&fttE56$V$s6%D$uEgf$Մ'5S$Wv$WD֤FtdeFDwtdEc&6euDdUfVCFc5'EŧutSTDd$dCWtťf34fœ%Ugtg%FEU4eVg5ESW&DD$vew&eUFeU#%'vƆV$eFfww%S5&Ħţ4$UD#e&%$sdftdUeDf%s%euFg%&%Ed5f%$u$u6##TSUVedd5v5SWvtSTeU$$W4&'#E3'6&Uw7&v$&efSUut6V%fVF#$gEetCWEW7vvƅtSSEDD5#uD%dSes5uSeuD$וf%3ŕvtgetgUfC%C%uDW7uf$&C%D&cdD%Fwuf5ufńDE7EVוEF$veee4ce&%&CC4EeFdUFv3$6eUecEu'6$V֥v3FTdev4VĤCE&TedUVf%$e'vFde5utU5CFc$UF5e%5Fd&$F&ׄFf#eV'ŅvFDdfdħ5uD'6VUVfėFUSuVF%%UVCeuegŖ$e$EDf#cEF'6CU4SŕUd&d6U$UgFUuef$&5V7ted%tV5#Vfe6ETEsWfCfӓ57#4e&Ťuw3FԤde6%w36Gu5tuffևUesT6&F5VW&F6Ee%EEU&Fd&c%C3UeeEdgg%vfDt4ӓEׇ6V'%EEeF$SU5Fׇtִ$Մ#6efc4su#&Edue$D&v%u3e&$gVCEe'VUu$fF&ťe74'UUw4edUEdu%efť6VuF6EusV5eքDd%&#5$W#DeeuCE%5feSf6Wu&e5W#DdvDTԅ$W#UwEu4ddvE$eEfg5#eFu%ecDdvDW&4SUEewce5$dWu6c%$5D'5eWef5V6ŤvŤ$eF&Efŕ&e4UդVu'%6VeD54S6E&5&Ĥe%FeUef6$Uev$U5W%tu$efօtģ5vf#$v$VEV&Ewv#ffFf%UUwvԧ5v%STEVDSTVFUsC%6UfdVCSuv%&$SWEw&Cef&'vuFgV5&ĤUW%d$SU5%eWde5VtԆvefg&C%eVdeV%5FŤu&UdU%Gueug%fUeUV$fTeef%DdUf׆FdfDee%gdEvcg%&u%wuvve%DeEUg5#'FTgfueSUf$VEF%Ew#EF%V%FĵVE4fFFDeEfƆ&$Uev$U5Vet%df4ee$Sf3%'E5ec%%VetWeDd5EUf$eW&dwED%5Vĥe5WE%fuUedddeFfUv6u&EeST5eedSW%FׄEee$SUfׅFV&6F37U#EdD'u$e%6f%DeFDSUF&C4#%wTfńDuC3FUSUf妅'SVGŖgVuV7WwueU4Uc#T#UV$Tg$gU%DdV%FTe$W&#Vvewtc#tז#7$vdTdׅF6VGfddtuFd#5eV$tFD%eFEEU$6$fDvF%wEe6Fv7ee%SS7%%3$VEF$fuedv$6օU$w5FGuutuV$evF6ffUU$fEUvDD$5tdev&dU&UWCe&Fdt5d6'%5tu%US'tu4dUtT%dD&ed$Du$W5FtdeFd֦DD6e'Fe$geV$eV$VV&7U4f&ec%'5V'uudu5DdF%WusVTSUe6&cusT53$vFv5VSU&#eF&UVD%$DwV3vEf&&SCF$gF$SfׅdstgFDDtwDŗdUfUve$eUeevUE6&ĤU'VՇ%fֆfD&w7fVFSfTEfvdwCv&UF$$vuS%gEeD&c&Guut$wFTf6ťeddfe$f4Duc5ewFEe&UfECfFGDԆ4#5esFEUf6ħEfF&ĥ%eT%vĦdF$V$ffUc5dV&$WCtEe&ee%uCEE&$TWF%6%deu&eeeDeesŤ%E&#UUw7%dgEEWEdԆ5UDDtestSTՖuF䅤dfcW&ģev5%Uc5VTfE%6VքEwDtdVs&Ĥev&$S$WfFU6憕%%esUSWV4f$%F'5Du#eDTc%E6uvĦ&uEUdUVƖ$dD4$ef36VVŗucETtו6EFdD%d7eu%Ede'4eVeeUE56g56e$ST5fg4g5&EEU5ee5&uD$te5UsC&d%W5v&g%&U$U5c3WgEecdeuf%fuVŤ4eV$SU54STUD$Vėu&ĥuegEwDu$tdfĤesVTteVDfքEDSUe4fvf3&4wUc$u$fC7F&EDd$U6%dS#VS6efևUwCFEdeUFEUsC&e%UeeVdutfu6e5tTWV$ueE$FusdSUe5Vօ5fׇ56ŗv4U5%WuUsC&e%%eeefdEe%e&U4Wvd%deef%%fĤ5DD$&ץ6euUe5Eg%f5eVUe5tU$%u$UUD4deu&ť6Sd#$ef4%vEDV4SvV%dV33%'E5etd5Vetf%fD#UfUuvv'ev5dgvSv4eUedV$fTeVƴUcEdee5Ftv%tg%U4de6Ӕ#Ue&EueV$e֤WfDutegFTvƖ$VvCfT%V&e5&5ef$SUeEU'5feUEeD#5dwedcefe֤WD&VEeVEfUwDvĥevƕdTvŧd%$fƄUdD$u$d4W4懅Ewu$eWEUeUc5V$dtee%EceVu&g%FueU5Fg5eu&DֳGUsT3$U55#6efS$Ddgĕ'6ԣeUD$UFef3U%d5ve&C#f$u%U5ׄew4$WF$vde&6E4ԄvS$v'VťeU4tet愥UeufGue%ef3VV%6&U'5FťSu&WEwF&V4EWudVG&TtWE6%V6TeWtՅdwV&Ufv6ƥeWedUcFFDE$sewUfuuEu$wef3uFDdcCėV&EVׄ5te64Tvv#%$sf7V5TSe%D&&EwG63eVe%UWfŤee&F$vUdVe#64sdvFg5tecevdu'UD5dWUeW5S'7&d%gedudee6%w#UvTSu6ťtևuVvDueuD%edg7$dg%v$FEsT$tV%eEveuTe%UUf65tFDeEUSW%D%e%U56%eUdŧFgfDWDԅ$eeV5$du6eEegUD$uedDԅ$W#&3t4v3%gFg&D'v7d$gv3V$gDugwGU&C4GU&C4&&#6w4ӗusS$tU#DDԴ4'Us&uEwt6E#VDucFDd6u#VDucFDF4tt6Edu&Ŧc$E'35&6vt4v&gEut&uEw6t4#3%g&uEwtcDvt4'Us&uEwtf&&T5g6gVD3##S&uEwtf&&T&UFu#U6'u6gU%g#ve3%g74'Us&uEwtcDvt4#3%gvt4vDs%sCducU6ֆ#$C&ĵSce5DdVǥ5s4gE5E%e5TegU&&VŦEw5tWFVED6VE4STUV%e%geFdu&%evU3VUWuS6䦵c5#$u$fW5#V4ffօֳS%f3VC&ƄvƄEgV׆UEguVV6cW5ufDԗffuf7$tץօgC&ĶUg&5#vFtSGuvƄDS4&#DT6%6׆%Eg5VEvVF$W5#V4ffօֳS%f3WwFׅ5TEfńwӗ#v&eF&tW5v׆Ԥf#UDe&5twc%6V$6ֆceudׄtDwDg%%ttE5%usvUtEf&tSf3FFוfƶ%wvDvƷDf&c%vUu'F$wWfuDF%D$%EfEfS6c%'u6$tEfDcWfg5%#4#VDDfD4SGu#vCFքGDvg6ftd564tTtVfD$fŅFTVV6cW5DfDvƷDf&cfG6#&$vFgEU%%W%vַtSG&DdwCev&DSSe5DcF&%SU5GvV#&Ƥ'eVEvDSUF׆&uuDvfF妵gWuC#T$f$#UvE&DfTDf5&ttgDE%V#UvF4tץV׆%e%DfD3%%F&tVCGvDudE%%F%VtE&DtSV5sGtצ6fŧ%FDD6Fąef3&VeC4%5%e4sVFW%UU$e#'f%EuU4UeVudwF%%VTddsTEe%4VE5WvesDdgFe5dSUvuvEutTSFTU%6344dcES$W&FceG&VGe6՗u$E&TST5d5EVDUdW%wTS6DDewEw%EcVǖ7U冃%USECdWfCdդd5%E'&D6ÖdTVD%uv6$#SFVe%ewfԣDdTuW$vw5E&v&eŤgu5%E$'%dU%6g4TeFդWdSUEu%f$SԄ%DֳT&F4wEE%e&S#e5ES66EDDdtDEV5DUt&Us&&GU#WEgs#%gEdwdԓ%C%%e#&F5wVEg$EteeG5FDDeTf&D֤VWEE$FE$vDdVEsVW5FTtue&ƅEtc6Uc%U$EEUWTguW4fFf%tFwVEe4V$vǅuw4t%sfWwe6'u$D5ESVS%gVDwU5W3De7FE䦥3#4uFfV#uEED$wE##WvV'D'7FǕĦG&$#5t3&fTsF#43TETg5EdU'$cc&DD3#CD6s ЦfW'6W&F"V6W7FW"vVVBcCW7FW&W@WFFF�SW7FW&W@�7V3FWF㠢�FSFVfV@�&Vv7G&FvUV7V3'V7F7G&&Vv7G&F6#ScVV3CssScVVCfFF#V33#3cc##3CcVScCFfS#V" v&vUV7V3'V7F7G&v&6#ScF6CF&6CCS3VV6SCf#S#cCCf&FCcf6CcV3&FfC3CSVV3cCssB 6W7FW$S'&Vv7FW&VB6W7FW"ӆ׆2 W76S&V6W7FW"vVVBvVB FU6VVCFW&F3VffV7C66VGVPWFR&RV&W&WFW2g&W&F#W7G0ЦfW'6cC6V7&W@WFFFS&V6W7FW"vVVB֖vRV7&VFVF2 W76S&V6W7FW"vVVBvVB GSV&W&WFW2F6W&6fv6FFF6W&6fv6Wvt4E&7d44t4EtcTdvWvt4t4c4d4c#tDEVEs&TtDuegvf6זs%dV##efEU4$WV&Ԥv6U%Uef#$T$u#5FEe$ev׆4gdfudv$SeWeVDtd&edgS7ֳvĥUd%%Ct4t4swvvt4vete